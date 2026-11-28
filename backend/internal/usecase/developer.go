package usecase

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/secret"
)

var (
	ErrAPIKeyNotFound          = errors.New("API key not found or revoked")
	ErrWebhookEndpointNotFound = errors.New("webhook endpoint not found")
	ErrWebhookDeliveryNotFound = errors.New("webhook delivery not found")
	ErrSSRFProtectionBlocked   = errors.New("webhook destination URL is rejected: private, loopback, or metadata addresses are prohibited")
	ErrInvalidWebhookURL       = errors.New("invalid webhook URL")
)

type DeveloperUsecase struct {
	devRepo repository.DeveloperRepository
	cipher  *secret.Cipher
}

func NewDeveloperUsecase(devRepo repository.DeveloperRepository, cipher *secret.Cipher) *DeveloperUsecase {
	return &DeveloperUsecase{
		devRepo: devRepo,
		cipher:  cipher,
	}
}

// DTOs

type CreateAPIKeyInput struct {
	ProjectID  string   `json:"project_id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	ExpiryDays int      `json:"expiry_days"` // 0 = never expires
}

type CreateAPIKeyResult struct {
	Key    *models.APIKey `json:"key"`
	RawKey string         `json:"raw_key"` // Shown only once
}

type CreateWebhookEndpointInput struct {
	ProjectID string   `json:"project_id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Enabled   *bool    `json:"enabled"`
}

type UpdateWebhookEndpointInput struct {
	Name    *string   `json:"name"`
	URL     *string   `json:"url"`
	Events  *[]string `json:"events"`
	Enabled *bool     `json:"enabled"`
}

// --- API Key Logic ---

func (u *DeveloperUsecase) CreateAPIKey(ctx context.Context, tenantID uuid.UUID, input CreateAPIKeyInput) (*CreateAPIKeyResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("API key name is required")
	}

	// Generate high-entropy 32-byte secret
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}
	rawSecret := hex.EncodeToString(randomBytes)
	rawKey := fmt.Sprintf("nv_live_%s", rawSecret)
	keyPrefix := fmt.Sprintf("nv_live_%s", rawSecret[:8])

	// Compute SHA-256 hash for secure storage
	hash := sha256.Sum256([]byte(rawKey))
	secretHash := hex.EncodeToString(hash[:])

	scopesJSON, err := json.Marshal(input.Scopes)
	if err != nil {
		scopesJSON = []byte("[]")
	}

	var expiresAt *time.Time
	if input.ExpiryDays > 0 {
		exp := time.Now().AddDate(0, 0, input.ExpiryDays)
		expiresAt = &exp
	}

	projID := input.ProjectID
	if projID == "" {
		projID = "default"
	}

	apiKey := &models.APIKey{
		TenantID:   tenantID,
		ProjectID:  projID,
		Name:       name,
		KeyPrefix:  keyPrefix,
		SecretHash: secretHash,
		Scopes:     string(scopesJSON),
		ExpiresAt:  expiresAt,
	}

	if err := u.devRepo.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, err
	}

	return &CreateAPIKeyResult{
		Key:    apiKey,
		RawKey: rawKey,
	}, nil
}

func (u *DeveloperUsecase) ListAPIKeys(ctx context.Context, tenantID uuid.UUID) ([]models.APIKey, error) {
	return u.devRepo.ListAPIKeysByTenant(ctx, tenantID)
}

func (u *DeveloperUsecase) RevokeAPIKey(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	_, err := u.devRepo.GetAPIKeyByID(ctx, tenantID, id)
	if err != nil {
		return ErrAPIKeyNotFound
	}
	return u.devRepo.RevokeAPIKey(ctx, tenantID, id)
}

func (u *DeveloperUsecase) ValidateRawAPIKey(ctx context.Context, rawKey string, requiredScope string) (*models.APIKey, error) {
	if !strings.HasPrefix(rawKey, "nv_live_") {
		return nil, ErrAPIKeyNotFound
	}

	hash := sha256.Sum256([]byte(rawKey))
	secretHash := hex.EncodeToString(hash[:])

	apiKey, err := u.devRepo.FindAPIKeyBySecretHash(ctx, secretHash)
	if err != nil || apiKey == nil {
		return nil, ErrAPIKeyNotFound
	}

	// Validate scope if specified
	if requiredScope != "" {
		var scopes []string
		_ = json.Unmarshal([]byte(apiKey.Scopes), &scopes)
		hasScope := false
		for _, s := range scopes {
			if s == requiredScope || s == "*" || strings.HasPrefix(requiredScope, strings.TrimSuffix(s, "*")) {
				hasScope = true
				break
			}
		}
		if !hasScope {
			return nil, errors.New("API key lacks required scope: " + requiredScope)
		}
	}

	// Update last used asynchronously
	go func(id uuid.UUID) {
		_ = u.devRepo.UpdateAPIKeyLastUsed(context.Background(), id)
	}(apiKey.ID)

	return apiKey, nil
}

// --- Webhooks & SSRF Protection ---

func isBlockedWebhookIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		ip.String() == "169.254.169.254"
}

// ValidatePublicWebhookURL validates that rawURL is an http/https URL resolving
// only to public routable IP addresses.
func ValidatePublicWebhookURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrInvalidWebhookURL
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return ErrInvalidWebhookURL
	}

	// Block loopback aliases
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return ErrSSRFProtectionBlocked
	}

	// Resolve IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil || len(ips) == 0 {
		return errors.New("cannot resolve webhook destination host")
	}

	for _, ip := range ips {
		if isBlockedWebhookIP(ip) {
			return ErrSSRFProtectionBlocked
		}
	}
	return nil
}

func (u *DeveloperUsecase) ValidateWebhookURL(rawURL string) error {
	return ValidatePublicWebhookURL(rawURL)
}

func (u *DeveloperUsecase) CreateWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, input CreateWebhookEndpointInput) (*models.WebhookEndpoint, error) {
	name := strings.TrimSpace(input.Name)
	rawURL := strings.TrimSpace(input.URL)
	if name == "" || rawURL == "" {
		return nil, errors.New("webhook name and URL are required")
	}

	if err := u.ValidateWebhookURL(rawURL); err != nil {
		return nil, err
	}

	// Generate signing secret
	randomSecret := make([]byte, 24)
	_, _ = rand.Read(randomSecret)
	secretHex := hex.EncodeToString(randomSecret)

	encSecret := secretHex
	if u.cipher != nil && u.cipher.Enabled() {
		enc, err := u.cipher.Encrypt(secretHex)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt webhook secret: %w", err)
		}
		encSecret = enc
	}

	projID := input.ProjectID
	if projID == "" {
		projID = "default"
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	ep := &models.WebhookEndpoint{
		TenantID:        tenantID,
		ProjectID:       projID,
		Name:            name,
		URL:             rawURL,
		SecretEncrypted: encSecret,
		Enabled:         enabled,
	}

	if err := u.devRepo.CreateWebhookEndpoint(ctx, ep); err != nil {
		return nil, err
	}

	for _, ev := range input.Events {
		sub := &models.WebhookSubscription{
			TenantID:   tenantID,
			EndpointID: ep.ID,
			EventType:  strings.TrimSpace(ev),
		}
		_ = u.devRepo.AddWebhookSubscription(ctx, sub)
	}

	return ep, nil
}

func (u *DeveloperUsecase) ListWebhookEndpoints(ctx context.Context, tenantID uuid.UUID) ([]models.WebhookEndpoint, error) {
	return u.devRepo.ListWebhookEndpointsByTenant(ctx, tenantID)
}

func (u *DeveloperUsecase) GetWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.WebhookEndpoint, error) {
	ep, err := u.devRepo.GetWebhookEndpointByID(ctx, tenantID, id)
	if err != nil {
		return nil, ErrWebhookEndpointNotFound
	}
	return ep, nil
}

func (u *DeveloperUsecase) UpdateWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, input UpdateWebhookEndpointInput) (*models.WebhookEndpoint, error) {
	ep, err := u.GetWebhookEndpoint(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		ep.Name = strings.TrimSpace(*input.Name)
	}
	if input.URL != nil && strings.TrimSpace(*input.URL) != "" {
		if err := u.ValidateWebhookURL(strings.TrimSpace(*input.URL)); err != nil {
			return nil, err
		}
		ep.URL = strings.TrimSpace(*input.URL)
	}
	if input.Enabled != nil {
		ep.Enabled = *input.Enabled
	}

	if err := u.devRepo.UpdateWebhookEndpoint(ctx, ep); err != nil {
		return nil, err
	}

	if input.Events != nil {
		// Update subscriptions
		for _, sub := range ep.Subscriptions {
			_ = u.devRepo.DeleteWebhookSubscription(ctx, tenantID, ep.ID, sub.EventType)
		}
		for _, ev := range *input.Events {
			sub := &models.WebhookSubscription{
				TenantID:   tenantID,
				EndpointID: ep.ID,
				EventType:  strings.TrimSpace(ev),
			}
			_ = u.devRepo.AddWebhookSubscription(ctx, sub)
		}
		if refreshed, err := u.GetWebhookEndpoint(ctx, tenantID, id); err == nil {
			ep = refreshed
		}
	}

	return ep, nil
}

func (u *DeveloperUsecase) DeleteWebhookEndpoint(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	_, err := u.GetWebhookEndpoint(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return u.devRepo.DeleteWebhookEndpoint(ctx, tenantID, id)
}

func (u *DeveloperUsecase) RotateWebhookSecret(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (string, error) {
	ep, err := u.GetWebhookEndpoint(ctx, tenantID, id)
	if err != nil {
		return "", err
	}

	randomSecret := make([]byte, 24)
	_, _ = rand.Read(randomSecret)
	newSecret := hex.EncodeToString(randomSecret)

	encSecret := newSecret
	if u.cipher != nil && u.cipher.Enabled() {
		enc, err := u.cipher.Encrypt(newSecret)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt webhook secret: %w", err)
		}
		encSecret = enc
	}

	ep.SecretEncrypted = encSecret
	if err := u.devRepo.UpdateWebhookEndpoint(ctx, ep); err != nil {
		return "", err
	}
	return newSecret, nil
}

func (u *DeveloperUsecase) TestWebhookPing(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.WebhookDelivery, error) {
	ep, err := u.GetWebhookEndpoint(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	testPayload := fmt.Sprintf(`{"id":"evt_%s","type":"ping","created_at":"%s","tenant_id":"%s","data":{"message":"Nevatal Cloud webhook test event"}}`,
		uuid.New().String()[:8], time.Now().UTC().Format(time.RFC3339), tenantID.String())

	delivery := &models.WebhookDelivery{
		TenantID:   tenantID,
		EndpointID: ep.ID,
		EventID:    fmt.Sprintf("evt_%s", uuid.New().String()[:8]),
		EventType:  "ping",
		Payload:    testPayload,
		Status:     "pending",
	}

	if err := u.devRepo.CreateWebhookDelivery(ctx, delivery); err != nil {
		return nil, err
	}

	// Dispatch a copy in the background to avoid mutating the returned struct concurrently.
	deliveryCopy := *delivery
	epCopy := *ep
	go u.DeliverWebhook(context.Background(), &deliveryCopy, &epCopy)

	return delivery, nil
}

func (u *DeveloperUsecase) ListDeliveries(ctx context.Context, tenantID uuid.UUID, endpointID *uuid.UUID, limit int) ([]models.WebhookDelivery, error) {
	return u.devRepo.ListWebhookDeliveries(ctx, tenantID, endpointID, limit)
}

func (u *DeveloperUsecase) RetryDelivery(ctx context.Context, tenantID uuid.UUID, deliveryID uuid.UUID) (*models.WebhookDelivery, error) {
	delivery, err := u.devRepo.GetWebhookDeliveryByID(ctx, tenantID, deliveryID)
	if err != nil {
		return nil, ErrWebhookDeliveryNotFound
	}
	ep, err := u.devRepo.GetWebhookEndpointByID(ctx, tenantID, delivery.EndpointID)
	if err != nil {
		return nil, ErrWebhookEndpointNotFound
	}

	delivery.Status = "pending"
	_ = u.devRepo.UpdateWebhookDelivery(ctx, delivery)

	deliveryCopy := *delivery
	epCopy := *ep
	go u.DeliverWebhook(context.Background(), &deliveryCopy, &epCopy)

	return delivery, nil
}

// --- Webhook Signature & Delivery Engine ---

func (u *DeveloperUsecase) DeliverWebhook(ctx context.Context, delivery *models.WebhookDelivery, ep *models.WebhookEndpoint) {
	startTime := time.Now()

	// 1. Re-validate URL for SSRF protection at dispatch time
	if err := u.ValidateWebhookURL(ep.URL); err != nil {
		delivery.Status = "failed"
		delivery.LastError = err.Error()
		_ = u.devRepo.UpdateWebhookDelivery(ctx, delivery)
		return
	}

	// 2. Compute HMAC-SHA256 signature
	rawSecret := ep.SecretEncrypted
	if u.cipher != nil && u.cipher.Enabled() {
		dec, err := u.cipher.Decrypt(ep.SecretEncrypted)
		if err != nil {
			delivery.Status = "failed"
			delivery.LastError = "failed to decrypt webhook signing secret"
			_ = u.devRepo.UpdateWebhookDelivery(ctx, delivery)
			return
		}
		rawSecret = dec
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(rawSecret))
	mac.Write([]byte(timestamp + "." + delivery.Payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	// 3. Send HTTP POST request using a DNS-rebinding-safe DialContext
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(dialCtx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("cannot resolve webhook destination host")
			}
			for _, ip := range ips {
				if isBlockedWebhookIP(ip) {
					return nil, ErrSSRFProtectionBlocked
				}
			}
			return dialer.DialContext(dialCtx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Prevent redirect-based SSRF
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", ep.URL, bytes.NewBufferString(delivery.Payload))
	if err != nil {
		delivery.Status = "failed"
		delivery.LastError = err.Error()
		_ = u.devRepo.UpdateWebhookDelivery(ctx, delivery)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Nevatal-Webhook-Dispatcher/1.0")
	req.Header.Set("X-Nevatal-Event-ID", delivery.EventID)
	req.Header.Set("X-Nevatal-Event", delivery.EventType)
	req.Header.Set("X-Nevatal-Timestamp", timestamp)
	req.Header.Set("X-Nevatal-Signature", fmt.Sprintf("v1=%s", signature))

	resp, err := client.Do(req)
	duration := int(time.Since(startTime).Milliseconds())

	attempt := &models.WebhookDeliveryAttempt{
		TenantID:       delivery.TenantID,
		DeliveryID:     delivery.ID,
		AttemptNumber:  delivery.AttemptCount + 1,
		ResponseTimeMS: duration,
	}

	if err != nil {
		delivery.Status = "failed"
		delivery.LastHTTPStatus = 0
		delivery.LastError = err.Error()
		attempt.HTTPStatus = 0
		attempt.ErrorMessage = err.Error()
	} else {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024)) // Read small body slice

		delivery.LastHTTPStatus = resp.StatusCode
		attempt.HTTPStatus = resp.StatusCode

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.Status = "delivered"
			now := time.Now()
			delivery.DeliveredAt = &now
			delivery.LastError = ""
		} else {
			delivery.Status = "failed"
			delivery.LastError = fmt.Sprintf("HTTP %d received from destination endpoint", resp.StatusCode)
			attempt.ErrorMessage = delivery.LastError
		}
	}

	now := time.Now()
	delivery.LastAttemptAt = &now
	delivery.AttemptCount++

	_ = u.devRepo.UpdateWebhookDelivery(ctx, delivery)
	_ = u.devRepo.RecordDeliveryAttempt(ctx, attempt)
}

// --- Event Outbox Publication ---

func (u *DeveloperUsecase) PublishEvent(ctx context.Context, tenantID uuid.UUID, projectID string, eventType string, aggregateType string, aggregateID string, data any) error {
	payloadBytes, err := json.Marshal(map[string]any{
		"id":         fmt.Sprintf("evt_%s", uuid.New().String()[:12]),
		"type":       eventType,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"tenant_id":  tenantID.String(),
		"project_id": projectID,
		"data":       data,
	})
	if err != nil {
		return err
	}

	outboxEvent := &models.EventOutbox{
		TenantID:      tenantID,
		ProjectID:     projectID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       string(payloadBytes),
	}

	if err := u.devRepo.SaveEventToOutbox(ctx, outboxEvent); err != nil {
		return err
	}

	// Asynchronously trigger matched webhook subscriptions
	go func() {
		endpoints, err := u.devRepo.ListWebhookEndpointsByTenant(context.Background(), tenantID)
		if err != nil {
			return
		}
		for _, ep := range endpoints {
			if !ep.Enabled {
				continue
			}
			matched := false
			for _, sub := range ep.Subscriptions {
				if sub.EventType == eventType || sub.EventType == "*" {
					matched = true
					break
				}
			}
			if matched {
				del := &models.WebhookDelivery{
					TenantID:   tenantID,
					EndpointID: ep.ID,
					EventID:    outboxEvent.ID.String(),
					EventType:  eventType,
					Payload:    outboxEvent.Payload,
					Status:     "pending",
				}
				if err := u.devRepo.CreateWebhookDelivery(context.Background(), del); err == nil {
					u.DeliverWebhook(context.Background(), del, &ep)
				}
			}
		}
		_ = u.devRepo.MarkOutboxEventPublished(context.Background(), outboxEvent.ID)
	}()

	return nil
}
