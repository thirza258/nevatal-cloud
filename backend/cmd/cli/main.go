package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	Version       = "v1.0.0"
	DefaultAPIURL = "http://localhost:8080"
)

type Config struct {
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
	Token  string `json:"token"`
}

func configFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".nevatal", "config.json")
}

func loadConfig() Config {
	cfg := Config{APIURL: DefaultAPIURL}
	data, err := os.ReadFile(configFilePath())
	if err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if envURL := os.Getenv("NEVATAL_API_URL"); envURL != "" {
		cfg.APIURL = envURL
	}
	if envKey := os.Getenv("NEVATAL_API_KEY"); envKey != "" {
		cfg.APIKey = envKey
	}
	if envToken := os.Getenv("NEVATAL_TOKEN"); envToken != "" {
		cfg.Token = envToken
	}
	return cfg
}

func saveConfig(cfg Config) error {
	p := configFilePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

type Client struct {
	BaseURL    string
	APIKey     string
	Token      string
	OutputMode string
	HTTPClient *http.Client
}

func (c *Client) doRequest(method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	} else if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) printOutput(data any, tableRenderer func()) {
	switch c.OutputMode {
	case "json":
		out, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(out))
	case "yaml":
		out, _ := yaml.Marshal(data)
		fmt.Println(string(out))
	default:
		if tableRenderer != nil {
			tableRenderer()
		} else {
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))
		}
	}
}

func parseGlobalFlags(args []string) (cfg Config, outputMode string, remaining []string) {
	cfg = loadConfig()
	outputMode = "table"

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" || arg == "--output" {
			if i+1 < len(args) {
				outputMode = args[i+1]
				i++
				continue
			}
		} else if strings.HasPrefix(arg, "--output=") {
			outputMode = strings.TrimPrefix(arg, "--output=")
			continue
		} else if arg == "--api-url" {
			if i+1 < len(args) {
				cfg.APIURL = args[i+1]
				i++
				continue
			}
		} else if strings.HasPrefix(arg, "--api-url=") {
			cfg.APIURL = strings.TrimPrefix(arg, "--api-url=")
			continue
		} else if arg == "--api-key" {
			if i+1 < len(args) {
				cfg.APIKey = args[i+1]
				i++
				continue
			}
		} else if strings.HasPrefix(arg, "--api-key=") {
			cfg.APIKey = strings.TrimPrefix(arg, "--api-key=")
			continue
		} else {
			remaining = append(remaining, arg)
		}
	}
	return cfg, outputMode, remaining
}

func printUsage() {
	fmt.Printf("Nevatal Cloud CLI %s\n\n", Version)
	fmt.Println("Usage: nevatal <command> [subcommand] [flags]")
	fmt.Println("\nGlobal Flags:")
	fmt.Println("  -o, --output <table|json|yaml>   Output format (default: table)")
	fmt.Println("      --api-url <url>              Nevatal Cloud API endpoint")
	fmt.Println("      --api-key <key>              API Key (nv_live_...)")
	fmt.Println("\nCommands:")
	fmt.Println("  login        Configure API URL and API Key / Token")
	fmt.Println("  logout       Clear stored authentication credentials")
	fmt.Println("  whoami       Show current authenticated tenant/user")
	fmt.Println("  version      Show CLI version")
	fmt.Println("  vm           Manage virtual machines (list, get, start, stop, delete)")
	fmt.Println("  network      Manage private networks and attachments (list, create, get, attach, delete)")
	fmt.Println("  firewall     Manage cloud firewalls and security rules (list, create, rule, delete)")
	fmt.Println("  lb           Manage load balancers and backend pools (list, create, get, delete)")
	fmt.Println("  dns          Manage DNS zones and records (list, create, records)")
	fmt.Println("  webhook      Manage webhook endpoints and deliveries (list, create, deliveries, ping)")
	fmt.Println("  job          Manage and monitor background infrastructure jobs (list, get)")
}

func main() {
	cfg, outputMode, args := parseGlobalFlags(os.Args[1:])

	if len(args) == 0 {
		printUsage()
		return
	}

	client := &Client{
		BaseURL:    cfg.APIURL,
		APIKey:     cfg.APIKey,
		Token:      cfg.Token,
		OutputMode: outputMode,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	cmd := args[0]
	subArgs := args[1:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("nevatal %s (Nevatal Cloud Developer CLI)\n", Version)

	case "login":
		handleLogin(cfg, subArgs)

	case "logout":
		cfg.APIKey = ""
		cfg.Token = ""
		if err := saveConfig(cfg); err != nil {
			fmt.Printf("Error saving configuration: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Successfully logged out.")

	case "whoami", "status":
		handleWhoami(client)

	case "vm", "instances":
		handleVM(client, subArgs)

	case "network", "net":
		handleNetwork(client, subArgs)

	case "firewall", "fw":
		handleFirewall(client, subArgs)

	case "lb", "loadbalancer":
		handleLoadBalancer(client, subArgs)

	case "dns":
		handleDNS(client, subArgs)

	case "webhook", "wh":
		handleWebhook(client, subArgs)

	case "job", "jobs":
		handleJob(client, subArgs)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s. Run 'nevatal --help' for usage.\n", cmd)
		os.Exit(1)
	}
}

func handleLogin(cfg Config, args []string) {
	var apiKey, apiURL string
	for i := 0; i < len(args); i++ {
		if args[i] == "--api-key" && i+1 < len(args) {
			apiKey = args[i+1]
			i++
		} else if args[i] == "--api-url" && i+1 < len(args) {
			apiURL = args[i+1]
			i++
		}
	}

	if apiURL != "" {
		cfg.APIURL = apiURL
	}
	if apiKey != "" {
		cfg.APIKey = apiKey
	} else if len(args) > 0 && strings.HasPrefix(args[0], "nv_live_") {
		cfg.APIKey = args[0]
	} else {
		fmt.Printf("Enter API Key [%s]: ", cfg.APIKey)
		var inputKey string
		_, _ = fmt.Scanln(&inputKey)
		if inputKey != "" {
			cfg.APIKey = inputKey
		}

		fmt.Printf("Enter API URL [%s]: ", cfg.APIURL)
		var inputURL string
		_, _ = fmt.Scanln(&inputURL)
		if inputURL != "" {
			cfg.APIURL = inputURL
		}
	}

	if err := saveConfig(cfg); err != nil {
		fmt.Printf("Failed to save credentials: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Configured credentials saved to %s\n", configFilePath())
}

func handleWhoami(client *Client) {
	body, err := client.doRequest("GET", "/api/v1/dashboard/summary", nil)
	if err != nil {
		fmt.Printf("Error checking identity: %v\n", err)
		os.Exit(1)
	}
	var res map[string]any
	_ = json.Unmarshal(body, &res)
	client.printOutput(res, func() {
		fmt.Println("Authenticated successfully to:", client.BaseURL)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Endpoint:\t%s\n", client.BaseURL)
		if client.APIKey != "" {
			fmt.Fprintf(w, "Auth Method:\tAPI Key (%s...)\n", client.APIKey[:min(12, len(client.APIKey))])
		} else if client.Token != "" {
			fmt.Fprintf(w, "Auth Method:\tBearer Token\n")
		}
		w.Flush()
	})
}

func handleVM(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/instances", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSTATUS\tIP ADDRESS\tVCPU\tRAM (MB)")
			for _, item := range list {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n",
					item["id"], item["name"], item["status"], item["ip_address"], item["vcpu"], item["ram_mb"])
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "get":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal vm get <vm-id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", "/api/v1/instances/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var item map[string]any
		_ = json.Unmarshal(body, &item)
		client.printOutput(item, nil)

	case "start":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal vm start <vm-id>")
			os.Exit(1)
		}
		_, err := client.doRequest("POST", "/api/v1/instances/"+args[1]+"/start", nil)
		if err != nil {
			fmt.Printf("Error starting VM: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("VM %s start job triggered.\n", args[1])

	case "stop":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal vm stop <vm-id>")
			os.Exit(1)
		}
		_, err := client.doRequest("POST", "/api/v1/instances/"+args[1]+"/stop", nil)
		if err != nil {
			fmt.Printf("Error stopping VM: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("VM %s stop job triggered.\n", args[1])

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal vm delete <vm-id>")
			os.Exit(1)
		}
		_, err := client.doRequest("DELETE", "/api/v1/instances/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error deleting VM: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("VM %s deletion initiated.\n", args[1])

	default:
		fmt.Printf("Unknown VM command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleNetwork(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/networks", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tCIDR\tATTACHMENTS\tCREATED")
			for _, item := range list {
				atts, _ := item["attachments"].([]any)
				fmt.Fprintf(w, "%v\t%v\t%v\t%d\t%v\n",
					item["id"], item["name"], item["cidr"], len(atts), item["created_at"])
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "create":
		var name, cidr, desc string
		for i := 1; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--cidr" && i+1 < len(args) {
				cidr = args[i+1]
				i++
			} else if args[i] == "--desc" && i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		}
		if name == "" || cidr == "" {
			fmt.Println("Usage: nevatal network create --name <name> --cidr <cidr> [--desc <desc>]")
			os.Exit(1)
		}
		body, err := client.doRequest("POST", "/api/v1/networks", map[string]string{
			"name":        name,
			"cidr":        cidr,
			"description": desc,
		})
		if err != nil {
			fmt.Printf("Error creating network: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("Private Network created: %s (%s)\n", res["name"], res["cidr"])
		})

	case "get":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal network get <id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", "/api/v1/networks/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var item map[string]any
		_ = json.Unmarshal(body, &item)
		client.printOutput(item, nil)

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal network delete <id>")
			os.Exit(1)
		}
		_, err := client.doRequest("DELETE", "/api/v1/networks/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error deleting network: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Network %s deleted.\n", args[1])

	case "attach":
		var netID, vmID, ip string
		if len(args) > 1 && !strings.HasPrefix(args[1], "--") {
			netID = args[1]
		}
		for i := 1; i < len(args); i++ {
			if args[i] == "--vm-id" && i+1 < len(args) {
				vmID = args[i+1]
				i++
			} else if args[i] == "--ip" && i+1 < len(args) {
				ip = args[i+1]
				i++
			}
		}
		if netID == "" || vmID == "" {
			fmt.Println("Usage: nevatal network attach <network-id> --vm-id <vm-id> [--ip <ip>]")
			os.Exit(1)
		}
		body, err := client.doRequest("POST", fmt.Sprintf("/api/v1/networks/%s/attachments", netID), map[string]string{
			"instance_id": vmID,
			"ip_address":  ip,
		})
		if err != nil {
			fmt.Printf("Error attaching VM to network: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("VM %s attached to network. Allocated IP: %v\n", vmID, res["ip_address"])
		})

	default:
		fmt.Printf("Unknown network command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleFirewall(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/firewalls", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tRULES COUNT\tCREATED")
			for _, item := range list {
				rules, _ := item["rules"].([]any)
				fmt.Fprintf(w, "%v\t%v\t%d\t%v\n", item["id"], item["name"], len(rules), item["created_at"])
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "create":
		var name, desc string
		for i := 1; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--desc" && i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		}
		if name == "" {
			fmt.Println("Usage: nevatal firewall create --name <name> [--desc <desc>]")
			os.Exit(1)
		}
		body, err := client.doRequest("POST", "/api/v1/firewalls", map[string]string{
			"name":        name,
			"description": desc,
		})
		if err != nil {
			fmt.Printf("Error creating firewall: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("Firewall created: %v (%v)\n", res["name"], res["id"])
		})

	case "rule":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal firewall rule <add|list|delete>")
			os.Exit(1)
		}
		if args[1] == "add" {
			var fwID, dir, act, proto, port, cidr, desc string
			if len(args) > 2 && !strings.HasPrefix(args[2], "--") {
				fwID = args[2]
			}
			for i := 2; i < len(args); i++ {
				switch args[i] {
				case "--fw-id":
					fwID = args[i+1]
					i++
				case "--direction":
					dir = args[i+1]
					i++
				case "--action":
					act = args[i+1]
					i++
				case "--protocol":
					proto = args[i+1]
					i++
				case "--port":
					port = args[i+1]
					i++
				case "--cidr":
					cidr = args[i+1]
					i++
				case "--desc":
					desc = args[i+1]
					i++
				}
			}
			if fwID == "" || dir == "" || act == "" || proto == "" {
				fmt.Println("Usage: nevatal firewall rule add <fw-id> --direction <inbound|outbound> --action <allow|deny> --protocol <tcp|udp|icmp|all> [--port <port>] [--cidr <cidr>]")
				os.Exit(1)
			}
			body, err := client.doRequest("POST", fmt.Sprintf("/api/v1/firewalls/%s/rules", fwID), map[string]any{
				"direction":   dir,
				"action":      act,
				"protocol":    proto,
				"port_range":  port,
				"cidr":        cidr,
				"description": desc,
			})
			if err != nil {
				fmt.Printf("Error adding rule: %v\n", err)
				os.Exit(1)
			}
			var res map[string]any
			_ = json.Unmarshal(body, &res)
			client.printOutput(res, func() {
				fmt.Printf("Firewall rule added to %s: %s %s %s %s\n", fwID, act, proto, port, cidr)
			})
		} else if args[1] == "delete" {
			if len(args) < 3 {
				fmt.Println("Usage: nevatal firewall rule delete <rule-id>")
				os.Exit(1)
			}
			_, err := client.doRequest("DELETE", "/api/v1/firewalls/rules/"+args[2], nil)
			if err != nil {
				fmt.Printf("Error deleting rule: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Firewall rule %s deleted.\n", args[2])
		}

	default:
		fmt.Printf("Unknown firewall command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleLoadBalancer(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/load-balancers", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSTATUS\tALGORITHM\tVIP\tLISTENERS\tPOOLS")
			for _, item := range list {
				listeners, _ := item["listeners"].([]any)
				pools, _ := item["pools"].([]any)
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%d\t%d\n",
					item["id"], item["name"], item["status"], item["algorithm"], item["vip_address"], len(listeners), len(pools))
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "create":
		var name, algo string
		for i := 1; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--algo" && i+1 < len(args) {
				algo = args[i+1]
				i++
			}
		}
		if name == "" {
			fmt.Println("Usage: nevatal lb create --name <name> [--algo <round_robin|least_conn|source_ip>]")
			os.Exit(1)
		}
		if algo == "" {
			algo = "round_robin"
		}
		body, err := client.doRequest("POST", "/api/v1/load-balancers", map[string]string{
			"name":      name,
			"algorithm": algo,
		})
		if err != nil {
			fmt.Printf("Error creating load balancer: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("Load Balancer created: %v (%v)\n", res["name"], res["id"])
		})

	case "get":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal lb get <id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", "/api/v1/load-balancers/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var item map[string]any
		_ = json.Unmarshal(body, &item)
		client.printOutput(item, nil)

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal lb delete <id>")
			os.Exit(1)
		}
		_, err := client.doRequest("DELETE", "/api/v1/load-balancers/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error deleting load balancer: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Load Balancer %s deleted.\n", args[1])

	default:
		fmt.Printf("Unknown lb command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleDNS(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/dns/zones", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tRECORDS\tCREATED")
			for _, item := range list {
				recs, _ := item["records"].([]any)
				fmt.Fprintf(w, "%v\t%v\t%d\t%v\n", item["id"], item["name"], len(recs), item["created_at"])
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "create":
		var name, desc string
		for i := 1; i < len(args); i++ {
			if args[i] == "--name" && i+1 < len(args) {
				name = args[i+1]
				i++
			} else if args[i] == "--desc" && i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		}
		if name == "" {
			fmt.Println("Usage: nevatal dns create --name <example.com> [--desc <desc>]")
			os.Exit(1)
		}
		body, err := client.doRequest("POST", "/api/v1/dns/zones", map[string]string{
			"name":        name,
			"description": desc,
		})
		if err != nil {
			fmt.Printf("Error creating DNS zone: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("DNS Zone created: %v (%v)\n", res["name"], res["id"])
		})

	case "records":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal dns records <zone-id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", fmt.Sprintf("/api/v1/dns/zones/%s/records", args[1]), nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tTYPE\tCONTENT\tTTL\tPRIORITY")
			for _, item := range list {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n",
					item["id"], item["name"], item["type"], item["content"], item["ttl"], item["priority"])
			}
			w.Flush()
		})

	default:
		fmt.Printf("Unknown dns command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleWebhook(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/webhooks", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tURL\tENABLED\tEVENTS\tSECRET PREVIEW")
			for _, item := range list {
				subs, _ := item["subscriptions"].([]any)
				var evts []string
				for _, s := range subs {
					if sm, ok := s.(map[string]any); ok {
						evts = append(evts, fmt.Sprint(sm["event_type"]))
					}
				}
				fmt.Fprintf(w, "%v\t%v\t%v\t%s\t%v\n",
					item["id"], item["url"], item["enabled"], strings.Join(evts, ","), item["secret_preview"])
			}
			w.Flush()
		})
		return
	}

	switch args[0] {
	case "create":
		var url, eventsStr, desc string
		for i := 1; i < len(args); i++ {
			if args[i] == "--url" && i+1 < len(args) {
				url = args[i+1]
				i++
			} else if args[i] == "--events" && i+1 < len(args) {
				eventsStr = args[i+1]
				i++
			} else if args[i] == "--desc" && i+1 < len(args) {
				desc = args[i+1]
				i++
			}
		}
		if url == "" {
			fmt.Println("Usage: nevatal webhook create --url <https://...> [--events <instance.created,instance.deleted>]")
			os.Exit(1)
		}
		events := []string{"*"}
		if eventsStr != "" {
			events = strings.Split(eventsStr, ",")
		}
		body, err := client.doRequest("POST", "/api/v1/webhooks", map[string]any{
			"url":         url,
			"events":      events,
			"description": desc,
		})
		if err != nil {
			fmt.Printf("Error creating webhook: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		client.printOutput(res, func() {
			fmt.Printf("Webhook endpoint created: %v\nSigning Secret: %v (Store this safely!)\n",
				res["url"], res["secret"])
		})

	case "deliveries":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal webhook deliveries <endpoint-id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", fmt.Sprintf("/api/v1/webhooks/%s/deliveries", args[1]), nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "DELIVERY ID\tEVENT TYPE\tSTATUS\tCODE\tATTEMPTS\tCREATED")
			for _, item := range list {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n",
					item["id"], item["event_type"], item["status"], item["response_code"], item["attempts"], item["created_at"])
			}
			w.Flush()
		})

	case "ping":
		if len(args) < 2 {
			fmt.Println("Usage: nevatal webhook ping <endpoint-id>")
			os.Exit(1)
		}
		body, err := client.doRequest("POST", fmt.Sprintf("/api/v1/webhooks/%s/ping", args[1]), nil)
		if err != nil {
			fmt.Printf("Error pinging webhook: %v\n", err)
			os.Exit(1)
		}
		var res map[string]any
		_ = json.Unmarshal(body, &res)
		fmt.Printf("Test ping delivered: Status %v (HTTP %v)\n", res["status"], res["response_code"])

	default:
		fmt.Printf("Unknown webhook command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleJob(client *Client, args []string) {
	if len(args) == 0 || args[0] == "list" {
		body, err := client.doRequest("GET", "/api/v1/jobs", nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var list []map[string]any
		_ = json.Unmarshal(body, &list)
		client.printOutput(list, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tACTION\tSTATUS\tPROGRESS\tCREATED")
			for _, item := range list {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v%%\t%v\n",
					item["id"], item["action"], item["status"], item["progress"], item["created_at"])
			}
			w.Flush()
		})
		return
	}

	if args[0] == "get" {
		if len(args) < 2 {
			fmt.Println("Usage: nevatal job get <job-id>")
			os.Exit(1)
		}
		body, err := client.doRequest("GET", "/api/v1/jobs/"+args[1], nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		var item map[string]any
		_ = json.Unmarshal(body, &item)
		client.printOutput(item, nil)
		return
	}

	fmt.Printf("Unknown job command: %s\n", args[0])
	os.Exit(1)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
