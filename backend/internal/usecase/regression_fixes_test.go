package usecase_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/usecase"
)

type stubStorageRepo struct {
	repository.StorageRepository
	bucket     *models.StorageBucket
	object     *models.StorageObject
	sharedLink *models.StorageSharedLink
	incErr     error
}

func (s *stubStorageRepo) CreateBucket(ctx context.Context, bucket *models.StorageBucket) error {
	return nil
}

func (s *stubStorageRepo) FindBucketByName(ctx context.Context, name string) (*models.StorageBucket, error) {
	if s.bucket != nil && s.bucket.Name == name {
		return s.bucket, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubStorageRepo) FindBucket(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageBucket, error) {
	if s.bucket != nil && s.bucket.ID == id && s.bucket.OwnerID == ownerID {
		return s.bucket, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubStorageRepo) FindBucketByID(ctx context.Context, id uuid.UUID) (*models.StorageBucket, error) {
	if s.bucket != nil && s.bucket.ID == id {
		return s.bucket, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubStorageRepo) FindObject(ctx context.Context, id uuid.UUID) (*models.StorageObject, error) {
	if s.object != nil && s.object.ID == id {
		return s.object, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubStorageRepo) CreateSharedLink(ctx context.Context, link *models.StorageSharedLink) error {
	return nil
}

func (s *stubStorageRepo) FindSharedLinkByToken(ctx context.Context, token string) (*models.StorageSharedLink, error) {
	if s.sharedLink != nil && s.sharedLink.ShareToken == token {
		return s.sharedLink, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubStorageRepo) IncrementSharedLinkDownloads(ctx context.Context, id uuid.UUID) error {
	return s.incErr
}

func TestStoragePathTraversalAndValidation(t *testing.T) {
	dir := t.TempDir()
	ownerID := uuid.New()
	bucket := &models.StorageBucket{
		ID:      uuid.New(),
		OwnerID: ownerID,
		Name:    "test-bucket",
	}
	obj := &models.StorageObject{
		ID:       uuid.New(),
		BucketID: bucket.ID,
		Key:      "hello.txt",
		Filename: "hello.txt",
	}
	repo := &stubStorageRepo{bucket: bucket, object: obj}
	uc := usecase.NewStorageUsecase(repo, dir)
	ctx := context.Background()

	// Bucket name traversal and invalid S3 names must fail.
	for _, bad := range []string{"../etc", "-leading", "trailing-", "bad_name", "ab", "bad.dot"} {
		if _, err := uc.CreateBucket(ctx, ownerID, usecase.CreateBucketInput{Name: bad}); err == nil {
			t.Fatalf("expected invalid bucket name %q to be rejected", bad)
		}
	}
	if b, err := uc.CreateBucket(ctx, ownerID, usecase.CreateBucketInput{Name: "valid-bucket-1"}); err != nil || b.Name != "valid-bucket-1" {
		t.Fatalf("expected valid-bucket-1 to succeed, got err=%v", err)
	}

	// Object and folder path traversal must fail.
	if _, err := uc.SaveObject(ctx, bucket, "../../outside", "file.txt", "text/plain", strings.NewReader("hi")); err == nil {
		t.Fatal("expected SaveObject folderPrefix traversal to fail")
	}
	if _, err := uc.SaveObject(ctx, bucket, "sub", "../file.txt", "text/plain", strings.NewReader("hi")); err == nil {
		t.Fatal("expected SaveObject filename traversal to fail")
	}
	if _, err := uc.CreateFolder(ctx, bucket, "sub", ".."); err == nil {
		t.Fatal("expected CreateFolder '..' to fail")
	}

	// Invalid ExpiresIn values must be rejected instead of defaulting silently.
	for _, exp := range []string{"garbage", "0s", "-2d", "-1h"} {
		if _, err := uc.CreateSharedLink(ctx, ownerID, usecase.CreateSharedLinkInput{
			ObjectID:   obj.ID,
			AccessType: "download",
			ExpiresIn:  exp,
		}); err == nil {
			t.Fatalf("expected CreateSharedLink with expires_in=%q to fail", exp)
		}
	}

	// Nil Object in SharedLink must not panic in OpenSharedLink.
	repo.sharedLink = &models.StorageSharedLink{
		ID:         uuid.New(),
		ShareToken: "tok-nil-obj",
		Object:     nil,
	}
	if _, _, err := uc.OpenSharedLink(ctx, "tok-nil-obj", ""); err == nil {
		t.Fatal("expected OpenSharedLink with nil Object to return an error")
	}
}

func TestValidatePublicWebhookURLBlocksUnsafeAddresses(t *testing.T) {
	blocked := []string{
		"http://0.0.0.0:8080/hook",
		"http://[::]:8080/hook",
		"http://10.0.0.1/internal",
		"http://192.168.1.50/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://api.localhost/hook",
		"ftp://example.com/hook",
		"",
	}
	for _, u := range blocked {
		if err := usecase.ValidatePublicWebhookURL(u); err == nil {
			t.Fatalf("expected URL %q to be blocked", u)
		}
	}
}

type stubPortRepo struct {
	repository.PortRepository
	mu        sync.Mutex
	allocated []int
}

func (s *stubPortRepo) ListAllocated(ctx context.Context, nodeName, protocol string) ([]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, len(s.allocated))
	copy(out, s.allocated)
	return out, nil
}

func TestPortUsecaseConcurrentNoCollision(t *testing.T) {
	uc := usecase.NewPortUsecase(&stubPortRepo{})
	ctx := context.Background()

	const workers = 10
	ports := make(chan int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, _, err := uc.FindAvailablePort(ctx, "minecraft_java", "node-1")
			if err != nil {
				t.Errorf("FindAvailablePort error: %v", err)
				return
			}
			ports <- p
		}()
	}
	wg.Wait()
	close(ports)

	seen := make(map[int]bool)
	var first int
	for p := range ports {
		if first == 0 {
			first = p
		}
		if seen[p] {
			t.Fatalf("duplicate concurrent port assigned: %d", p)
		}
		seen[p] = true
	}
	if len(seen) != workers {
		t.Fatalf("expected %d distinct ports, got %d", workers, len(seen))
	}

	// Releasing a pending port allows re-use.
	uc.ReleasePendingPort("node-1", "tcp", first)
	reused, _, err := uc.FindAvailablePort(ctx, "minecraft_java", "node-1")
	if err != nil {
		t.Fatalf("FindAvailablePort after release failed: %v", err)
	}
	if reused != first {
		t.Fatalf("expected released port %d to be reused, got %d", first, reused)
	}
}

type stubLBRepo struct {
	repository.LoadBalancerRepository
	lb   *models.LoadBalancer
	pool *models.LoadBalancerPool
}

func (s *stubLBRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*models.LoadBalancer, error) {
	if s.lb != nil && s.lb.ID == id && s.lb.TenantID == tenantID {
		return s.lb, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubLBRepo) GetPoolByID(ctx context.Context, tenantID, poolID uuid.UUID) (*models.LoadBalancerPool, error) {
	if s.pool != nil && s.pool.ID == poolID && s.pool.TenantID == tenantID {
		return s.pool, nil
	}
	return nil, repository.ErrNotFound
}

type stubInstRepo struct {
	repository.InstanceRepository
	inst *models.Instance
}

func (s *stubInstRepo) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.Instance, error) {
	if s.inst != nil && s.inst.ID == id && (ownerID == nil || s.inst.OwnerID == *ownerID) {
		return s.inst, nil
	}
	return nil, repository.ErrNotFound
}

func TestLoadBalancerValidationRejectsInjectionAndCrossLB(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	lbID := uuid.New()
	otherLBID := uuid.New()
	poolID := uuid.New()
	instID := uuid.New()

	lbRepo := &stubLBRepo{
		lb: &models.LoadBalancer{ID: lbID, TenantID: tenantID, Name: "web"},
		pool: &models.LoadBalancerPool{
			ID:             poolID,
			TenantID:       tenantID,
			LoadBalancerID: otherLBID,
			Name:           "foreign-pool",
		},
	}
	privIP := "10.20.0.5"
	instRepo := &stubInstRepo{
		inst: &models.Instance{ID: instID, OwnerID: tenantID, PrivateIP: &privIP},
	}

	uc := usecase.NewLoadBalancerUsecase(lbRepo, instRepo, nil)

	// Cross-LB pool attachment on AddListener must fail.
	if _, err := uc.AddListener(ctx, tenantID, lbID, usecase.CreateListenerInput{
		Protocol:      "http",
		FrontendPort:  80,
		DefaultPoolID: &poolID,
	}); err == nil {
		t.Fatal("expected AddListener with pool from another LB to fail")
	}

	// Point pool to lbID, then test invalid IP / HAProxy config injection in AddMember.
	lbRepo.pool.LoadBalancerID = lbID
	if _, err := uc.AddMember(ctx, tenantID, lbID, poolID, usecase.AddMemberInput{
		InstanceID: instID,
		Address:    "10.0.0.1\nserver evil 127.0.0.1:80 check",
		Port:       80,
	}); err == nil {
		t.Fatal("expected AddMember with invalid/injected IP address to fail")
	}
}

type stubQuotaRepo struct {
	repository.QuotaRepository
	q *models.ResourceQuota
}

func (s *stubQuotaRepo) FindByUserID(ctx context.Context, userID uuid.UUID) (*models.ResourceQuota, error) {
	return s.q, nil
}

type stubQuotaInstRepo struct {
	repository.InstanceRepository
	tot repository.ResourceTotals
}

func (s *stubQuotaInstRepo) SumUsageByOwner(ctx context.Context, ownerID uuid.UUID) (repository.ResourceTotals, error) {
	return s.tot, nil
}

type stubQuotaResRepo struct {
	repository.ReservationRepository
	tot repository.ResourceTotals
}

func (s *stubQuotaResRepo) SumActiveByOwner(ctx context.Context, ownerID uuid.UUID) (repository.ResourceTotals, error) {
	return s.tot, nil
}

func TestQuotaEnforcesMaxInstancesWithReservations(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	uc := usecase.NewQuotaUsecase(
		&stubQuotaRepo{q: &models.ResourceQuota{
			UserID:       userID,
			MaxVCPUs:     16,
			MaxMemoryMB:  32768,
			MaxStorageGB: 500,
			MaxInstances: 2,
		}},
		&stubQuotaInstRepo{tot: repository.ResourceTotals{Count: 1, TotalVCPUs: 2, TotalMemory: 2048, TotalStorage: 20}},
		&stubQuotaResRepo{tot: repository.ResourceTotals{Count: 1, TotalVCPUs: 2, TotalMemory: 2048, TotalStorage: 20}},
	)

	err := uc.CheckQuota(ctx, userID, 1, 1024, 10)
	if err == nil || !strings.Contains(err.Error(), "instances") {
		t.Fatalf("expected MaxInstances quota error, got: %v", err)
	}
}

type stubDevRepo struct {
	repository.DeveloperRepository
	delivery *models.WebhookDelivery
	ep       *models.WebhookEndpoint
	updated  *models.WebhookDelivery
}

func (s *stubDevRepo) GetWebhookDeliveryByID(ctx context.Context, tenantID, deliveryID uuid.UUID) (*models.WebhookDelivery, error) {
	if s.delivery != nil && s.delivery.ID == deliveryID && s.delivery.TenantID == tenantID {
		return s.delivery, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubDevRepo) GetWebhookEndpointByID(ctx context.Context, tenantID, id uuid.UUID) (*models.WebhookEndpoint, error) {
	if s.ep != nil && s.ep.ID == id && s.ep.TenantID == tenantID {
		return s.ep, nil
	}
	return nil, repository.ErrNotFound
}

func (s *stubDevRepo) UpdateWebhookDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	cp := *delivery
	s.updated = &cp
	return nil
}

func (s *stubDevRepo) RecordDeliveryAttempt(ctx context.Context, attempt *models.WebhookDeliveryAttempt) error {
	return nil
}

func TestDeveloperRetryDeliveryDoesNotDoubleIncrementAttemptCount(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	epID := uuid.New()
	delID := uuid.New()

	repo := &stubDevRepo{
		ep: &models.WebhookEndpoint{
			ID:              epID,
			TenantID:        tenantID,
			Name:            "hook",
			URL:             "https://example.com/hook",
			SecretEncrypted: "secret",
		},
		delivery: &models.WebhookDelivery{
			ID:           delID,
			TenantID:     tenantID,
			EndpointID:   epID,
			Status:       "failed",
			AttemptCount: 1,
		},
	}

	uc := usecase.NewDeveloperUsecase(repo, nil)
	del, err := uc.RetryDelivery(ctx, tenantID, delID)
	if err != nil {
		t.Fatalf("RetryDelivery failed: %v", err)
	}
	if del.AttemptCount != 1 {
		t.Fatalf("expected RetryDelivery not to pre-increment AttemptCount before DeliverWebhook, got %d", del.AttemptCount)
	}
	// Allow background DeliverWebhook (which will fail quickly or resolve) a brief moment not to race after test exit.
	time.Sleep(10 * time.Millisecond)
}
