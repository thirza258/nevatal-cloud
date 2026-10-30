package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nevatal/cloud/internal/config"
)

type httpPVEClient struct {
	baseURL    string
	httpClient *http.Client
	authHeader string
}

func NewClient(cfg *config.Config) ProxmoxClient {
	if cfg.ProxmoxMock {
		return NewMockClient(cfg.ProxmoxNodeName)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !cfg.ProxmoxVerifyTLS,
		},
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	baseURL := fmt.Sprintf("https://%s:%s/api2/json", cfg.ProxmoxHost, cfg.ProxmoxPort)
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", cfg.ProxmoxTokenID, cfg.ProxmoxTokenSecret)

	return &httpPVEClient{
		baseURL:    baseURL,
		httpClient: client,
		authHeader: authHeader,
	}
}

func (c *httpPVEClient) doRequest(ctx context.Context, method, endpoint string, body url.Values) ([]byte, error) {
	targetURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	var reqBody io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
		reqBody = strings.NewReader(body.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request failed: %w", err)
	}

	req.Header.Set("Authorization", c.authHeader)
	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxmox api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading proxmox response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("proxmox api returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

func (c *httpPVEClient) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/status", node), nil)
	if err != nil {
		return nil, err
	}

	var res PVEApiResponse[NodeStatus]
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("unmarshal node status failed: %w", err)
	}
	return &res.Data, nil
}

func (c *httpPVEClient) GetStorage(ctx context.Context, node string) ([]StorageInfo, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/storage", node), nil)
	if err != nil {
		return nil, err
	}

	var res PVEApiResponse[[]StorageInfo]
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("unmarshal storage info failed: %w", err)
	}
	return res.Data, nil
}

func (c *httpPVEClient) GetNextVMID(ctx context.Context) (int, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/cluster/nextid", nil)
	if err != nil {
		return 0, err
	}

	var res struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return 0, fmt.Errorf("unmarshal nextid failed: %w", err)
	}

	vmid, err := strconv.Atoi(res.Data)
	if err != nil {
		return 0, fmt.Errorf("parse nextid int failed: %w", err)
	}
	return vmid, nil
}

func (c *httpPVEClient) CloneVM(ctx context.Context, node string, templateVMID, newVMID int, name, targetStorage string) (string, error) {
	params := url.Values{}
	params.Set("newid", strconv.Itoa(newVMID))
	params.Set("name", name)
	params.Set("full", "1")
	if targetStorage != "" {
		params.Set("storage", targetStorage)
	}

	data, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/clone", node, templateVMID), params)
	if err != nil {
		return "", err
	}

	var res struct {
		Data string `json:"data"` // UPID
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return "", fmt.Errorf("unmarshal clone upid failed: %w", err)
	}
	return res.Data, nil
}

func (c *httpPVEClient) ConfigVM(ctx context.Context, node string, vmid int, cfg VMConfig) error {
	params := url.Values{}
	if cfg.Cores > 0 {
		params.Set("cores", strconv.Itoa(cfg.Cores))
	}
	if cfg.MemoryMB > 0 {
		params.Set("memory", strconv.Itoa(cfg.MemoryMB))
	}
	if cfg.CIUser != "" {
		params.Set("ciuser", cfg.CIUser)
	}
	if cfg.SSHPublicKey != "" {
		params.Set("sshkeys", url.QueryEscape(cfg.SSHPublicKey))
	}
	if cfg.IPConfig0 != "" {
		params.Set("ipconfig0", cfg.IPConfig0)
	} else {
		params.Set("ipconfig0", "ip=dhcp")
	}
	if cfg.Agent > 0 {
		params.Set("agent", strconv.Itoa(cfg.Agent))
	}

	_, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid), params)
	return err
}

func (c *httpPVEClient) ResizeDisk(ctx context.Context, node string, vmid int, disk string, sizeGB int) error {
	params := url.Values{}
	params.Set("disk", disk)
	params.Set("size", fmt.Sprintf("+%dG", sizeGB))

	_, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/resize", node, vmid), params)
	return err
}

func (c *httpPVEClient) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	data, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/status/start", node, vmid), nil)
	if err != nil {
		return "", err
	}
	var res struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(data, &res)
	return res.Data, nil
}

func (c *httpPVEClient) ShutdownVM(ctx context.Context, node string, vmid int) (string, error) {
	data, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", node, vmid), nil)
	if err != nil {
		return "", err
	}
	var res struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(data, &res)
	return res.Data, nil
}

func (c *httpPVEClient) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	data, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", node, vmid), nil)
	if err != nil {
		return "", err
	}
	var res struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(data, &res)
	return res.Data, nil
}

func (c *httpPVEClient) ResetVM(ctx context.Context, node string, vmid int) (string, error) {
	data, err := c.doRequest(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/status/reboot", node, vmid), nil)
	if err != nil {
		return "", err
	}
	var res struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(data, &res)
	return res.Data, nil
}

func (c *httpPVEClient) DeleteVM(ctx context.Context, node string, vmid int) (string, error) {
	params := url.Values{}
	params.Set("purge", "1")
	params.Set("destroy-unreferenced-disks", "1")

	data, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid), params)
	if err != nil {
		return "", err
	}
	var res struct {
		Data string `json:"data"`
	}
	_ = json.Unmarshal(data, &res)
	return res.Data, nil
}

func (c *httpPVEClient) GetVMStatus(ctx context.Context, node string, vmid int) (*VMCurrentStatus, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid), nil)
	if err != nil {
		return nil, err
	}

	var res PVEApiResponse[VMCurrentStatus]
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("unmarshal vm status failed: %w", err)
	}
	return &res.Data, nil
}

func (c *httpPVEClient) GetVMInterfaces(ctx context.Context, node string, vmid int) ([]NetworkInterface, error) {
	data, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid), nil)
	if err != nil {
		return nil, err
	}

	var res struct {
		Data struct {
			Result []NetworkInterface `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("unmarshal network interfaces failed: %w", err)
	}
	return res.Data.Result, nil
}

func (c *httpPVEClient) WaitForTask(ctx context.Context, node string, upid string) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for proxmox task %s", upid)
		case <-ticker.C:
			endpoint := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, url.PathEscape(upid))
			data, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				// Retry on transient network errors
				continue
			}

			var res struct {
				Data struct {
					Status     string  `json:"status"`
					ExitStatus *string `json:"exitstatus"`
				} `json:"data"`
			}
			if err := json.Unmarshal(data, &res); err != nil {
				continue
			}

			if res.Data.Status == "stopped" {
				if res.Data.ExitStatus != nil && *res.Data.ExitStatus == "OK" {
					return nil
				}
				exitErr := "unknown error"
				if res.Data.ExitStatus != nil {
					exitErr = *res.Data.ExitStatus
				}
				return fmt.Errorf("proxmox task failed with status: %s", exitErr)
			}
		}
	}
}
