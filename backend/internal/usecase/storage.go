package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// StorageUsecase implements S3-style object storage on the local filesystem
// (one directory per bucket under basePath) with metadata in the database.
type StorageUsecase struct {
	storage  repository.StorageRepository
	basePath string
}

func NewStorageUsecase(storage repository.StorageRepository, basePath string) *StorageUsecase {
	if basePath == "" {
		basePath = "/data/storage"
	}
	os.MkdirAll(basePath, 0755)
	return &StorageUsecase{
		storage:  storage,
		basePath: basePath,
	}
}

var (
	errBucketNotFound       = newError(KindNotFound, "NOT_FOUND", "Bucket not found")
	errObjectNotFound       = newError(KindNotFound, "NOT_FOUND", "Object not found")
	errObjectAccessDenied   = newError(KindForbidden, "FORBIDDEN", "Access denied")
	errObjectFileMissing    = newError(KindNotFound, "FILE_NOT_FOUND", "File not found on storage")
	errSharedFileMissing    = newError(KindNotFound, "FILE_NOT_FOUND", "File not found")
	errSharedPasswordNeeded = newError(KindUnauthorized, "PASSWORD_REQUIRED", "This shared link requires a password")
	errSharedPasswordWrong  = newError(KindUnauthorized, "INVALID_PASSWORD", "Invalid link password")
)

type CreateBucketInput struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	VersioningEnabled bool   `json:"versioning_enabled"`
	Region            string `json:"region"`
}

type UpdateBucketInput struct {
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	VersioningEnabled *bool  `json:"versioning_enabled"`
}

type CreateAccessKeyInput struct {
	Description string `json:"description"`
	Permissions string `json:"permissions"`
	BucketScope string `json:"bucket_scope"`
}

type CreateSharedLinkInput struct {
	ObjectID     uuid.UUID `json:"object_id"`
	AccessType   string    `json:"access_type"`
	Password     string    `json:"password"`
	ExpiresIn    string    `json:"expires_in"` // e.g. "1h", "24h", "7d", "30d"
	MaxDownloads *int64    `json:"max_downloads"`
}

type StorageOverviewResponse struct {
	TotalBuckets    int64 `json:"total_buckets"`
	TotalObjects    int64 `json:"total_objects"`
	TotalSizeBytes  int64 `json:"total_size_bytes"`
	TotalAccessKeys int64 `json:"total_access_keys"`
}

// GetOverview totals the owner's buckets, objects, bytes and active access
// keys. Query failures count as zero.
func (s *StorageUsecase) GetOverview(ctx context.Context, ownerID uuid.UUID) (*StorageOverviewResponse, error) {
	totalBuckets, _ := s.storage.CountActiveBuckets(ctx, ownerID)
	totalObjects, totalSizeBytes, _ := s.storage.SumObjects(ctx, ownerID)
	totalKeys, _ := s.storage.CountActiveAccessKeys(ctx, ownerID)

	return &StorageOverviewResponse{
		TotalBuckets:    totalBuckets,
		TotalObjects:    totalObjects,
		TotalSizeBytes:  totalSizeBytes,
		TotalAccessKeys: totalKeys,
	}, nil
}

func (s *StorageUsecase) ListBuckets(ctx context.Context, ownerID uuid.UUID) ([]models.StorageBucket, error) {
	return s.storage.ListBuckets(ctx, ownerID)
}

// GetBucket returns a live bucket owned by ownerID.
func (s *StorageUsecase) GetBucket(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) (*models.StorageBucket, error) {
	return s.storage.FindBucket(ctx, id, ownerID)
}

// GetOwnedBucket is GetBucket for API callers: a missing or foreign bucket is
// reported as NOT_FOUND "Bucket not found".
func (s *StorageUsecase) GetOwnedBucket(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) (*models.StorageBucket, error) {
	bucket, err := s.storage.FindBucket(ctx, id, ownerID)
	if err != nil {
		return nil, errBucketNotFound
	}
	return bucket, nil
}

func (s *StorageUsecase) GetBucketByName(ctx context.Context, name string) (*models.StorageBucket, error) {
	return s.storage.FindBucketByName(ctx, name)
}

func (s *StorageUsecase) CreateBucket(ctx context.Context, ownerID uuid.UUID, input CreateBucketInput) (*models.StorageBucket, error) {
	ctx = context.WithoutCancel(ctx)

	// Sanitize name according to S3 rules (lowercase, alphanumeric, hyphens)
	name := strings.ToLower(strings.TrimSpace(input.Name))
	if len(name) < 3 || len(name) > 63 {
		return nil, fmt.Errorf("bucket name must be between 3 and 63 characters")
	}

	if _, err := s.storage.FindBucketByName(ctx, name); err == nil {
		return nil, fmt.Errorf("bucket name '%s' is already taken", name)
	}

	region := input.Region
	if region == "" {
		region = "id-central-1"
	}
	visibility := input.Visibility
	if visibility == "" {
		visibility = "private"
	}

	bucket := models.StorageBucket{
		OwnerID:           ownerID,
		Name:              name,
		Description:       input.Description,
		Visibility:        visibility,
		VersioningEnabled: input.VersioningEnabled,
		Region:            region,
	}

	if err := s.storage.CreateBucket(ctx, &bucket); err != nil {
		return nil, err
	}

	bucketDir := filepath.Join(s.basePath, bucket.Name)
	_ = os.MkdirAll(bucketDir, 0755)

	return &bucket, nil
}

func (s *StorageUsecase) UpdateBucket(ctx context.Context, id uuid.UUID, ownerID uuid.UUID, input UpdateBucketInput) (*models.StorageBucket, error) {
	ctx = context.WithoutCancel(ctx)

	bucket, err := s.GetBucket(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}

	if input.Description != "" {
		bucket.Description = input.Description
	}
	if input.Visibility != "" {
		bucket.Visibility = input.Visibility
	}
	if input.VersioningEnabled != nil {
		bucket.VersioningEnabled = *input.VersioningEnabled
	}

	if err := s.storage.SaveBucket(ctx, bucket); err != nil {
		return nil, err
	}
	return bucket, nil
}

func (s *StorageUsecase) DeleteBucket(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)

	bucket, err := s.GetBucket(ctx, id, ownerID)
	if err != nil {
		return err
	}

	count, _ := s.storage.CountObjects(ctx, bucket.ID)
	if count > 0 {
		return fmt.Errorf("bucket is not empty (%d objects found). Delete objects first", count)
	}

	now := time.Now()
	bucket.DeletedAt = &now
	if err := s.storage.SaveBucket(ctx, bucket); err != nil {
		return err
	}

	bucketDir := filepath.Join(s.basePath, bucket.Name)
	_ = os.RemoveAll(bucketDir)

	return nil
}

// ListObjects lists a bucket the caller owns (NOT_FOUND "Bucket not found"
// otherwise), optionally under a folder prefix.
func (s *StorageUsecase) ListObjects(ctx context.Context, bucketID, ownerID uuid.UUID, prefix string) ([]models.StorageObject, error) {
	// Verify bucket ownership
	if _, err := s.GetOwnedBucket(ctx, bucketID, ownerID); err != nil {
		return nil, err
	}
	return s.storage.ListObjects(ctx, bucketID, prefix)
}

// GetObject returns an object by id regardless of owner.
func (s *StorageUsecase) GetObject(ctx context.Context, id uuid.UUID) (*models.StorageObject, error) {
	return s.storage.FindObject(ctx, id)
}

// getOwnedObject returns an object whose bucket the caller owns:
// NOT_FOUND "Object not found" when it does not exist, FORBIDDEN "Access
// denied" when it belongs to another tenant.
func (s *StorageUsecase) getOwnedObject(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageObject, error) {
	obj, err := s.storage.FindObject(ctx, id)
	if err != nil {
		return nil, errObjectNotFound
	}
	// Verify bucket ownership
	if _, err := s.storage.FindBucket(ctx, obj.BucketID, ownerID); err != nil {
		return nil, errObjectAccessDenied
	}
	return obj, nil
}

// OpenObject opens an object the caller owns for download. The caller closes
// the returned reader.
func (s *StorageUsecase) OpenObject(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageObject, io.ReadCloser, error) {
	obj, err := s.getOwnedObject(ctx, id, ownerID)
	if err != nil {
		return nil, nil, err
	}
	file, err := s.OpenObjectFile(ctx, obj)
	if err != nil {
		return nil, nil, errObjectFileMissing
	}
	return obj, file, nil
}

func (s *StorageUsecase) SaveObject(ctx context.Context, bucket *models.StorageBucket, folderPrefix, filename, contentType string, reader io.Reader) (*models.StorageObject, error) {
	ctx = context.WithoutCancel(ctx)

	cleanPrefix := strings.Trim(folderPrefix, "/")
	var key string
	if cleanPrefix != "" {
		key = cleanPrefix + "/" + filename
	} else {
		key = filename
	}

	targetDir := filepath.Join(s.basePath, bucket.Name, cleanPrefix)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	targetPath := filepath.Join(s.basePath, bucket.Name, key)
	dst, err := os.Create(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(dst, hasher)
	sizeBytes, err := io.Copy(multiWriter, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write object data: %w", err)
	}

	etag := hex.EncodeToString(hasher.Sum(nil))[:16]

	existing, err := s.storage.FindObjectByKey(ctx, bucket.ID, key)
	if err == nil {
		// Update existing
		oldSize := existing.SizeBytes
		existing.SizeBytes = sizeBytes
		existing.ContentType = contentType
		existing.ETag = etag
		existing.UpdatedAt = time.Now()
		if err := s.storage.SaveObject(ctx, existing); err != nil {
			return nil, err
		}

		bucket.TotalSizeBytes += (sizeBytes - oldSize)
		_ = s.storage.UpdateBucketSize(ctx, bucket, bucket.TotalSizeBytes)
		return existing, nil
	}

	publicURL := ""
	if bucket.Visibility == "public" {
		publicURL = fmt.Sprintf("https://s3.nevatal.id/%s/%s", bucket.Name, key)
	}

	obj := models.StorageObject{
		BucketID:     bucket.ID,
		Key:          key,
		Filename:     filename,
		FolderPrefix: cleanPrefix,
		ContentType:  contentType,
		SizeBytes:    sizeBytes,
		ETag:         etag,
		IsFolder:     false,
		PublicURL:    publicURL,
	}

	if err := s.storage.CreateObject(ctx, &obj); err != nil {
		return nil, err
	}

	// Update bucket totals
	bucket.TotalObjects++
	bucket.TotalSizeBytes += sizeBytes
	_ = s.storage.UpdateBucketTotals(ctx, bucket, bucket.TotalObjects, bucket.TotalSizeBytes)

	return &obj, nil
}

func (s *StorageUsecase) CreateFolder(ctx context.Context, bucket *models.StorageBucket, folderPrefix, folderName string) (*models.StorageObject, error) {
	ctx = context.WithoutCancel(ctx)

	cleanPrefix := strings.Trim(folderPrefix, "/")
	cleanName := strings.Trim(folderName, "/")
	var key string
	if cleanPrefix != "" {
		key = cleanPrefix + "/" + cleanName + "/"
	} else {
		key = cleanName + "/"
	}

	if existing, err := s.storage.FindObjectByKey(ctx, bucket.ID, key); err == nil {
		return existing, nil
	}

	obj := models.StorageObject{
		BucketID:     bucket.ID,
		Key:          key,
		Filename:     cleanName,
		FolderPrefix: cleanPrefix,
		ContentType:  "application/x-directory",
		SizeBytes:    0,
		IsFolder:     true,
	}

	if err := s.storage.CreateObject(ctx, &obj); err != nil {
		return nil, err
	}

	diskDir := filepath.Join(s.basePath, bucket.Name, cleanPrefix, cleanName)
	_ = os.MkdirAll(diskDir, 0755)

	return &obj, nil
}

// DeleteObject deletes an object the caller owns (see getOwnedObject for the
// errors), its file and its share of the bucket totals.
func (s *StorageUsecase) DeleteObject(ctx context.Context, id, ownerID uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)

	if _, err := s.getOwnedObject(ctx, id, ownerID); err != nil {
		return err
	}

	obj, err := s.storage.FindObject(ctx, id)
	if err != nil {
		return err
	}

	if bucket, err := s.storage.FindBucketByID(ctx, obj.BucketID); err == nil {
		if !obj.IsFolder {
			bucket.TotalObjects--
			bucket.TotalSizeBytes -= obj.SizeBytes
			if bucket.TotalSizeBytes < 0 {
				bucket.TotalSizeBytes = 0
			}
			_ = s.storage.UpdateBucketTotals(ctx, bucket, bucket.TotalObjects, bucket.TotalSizeBytes)
		}
		filePath := filepath.Join(s.basePath, bucket.Name, obj.Key)
		_ = os.RemoveAll(filePath)
	}

	return s.storage.DeleteObject(ctx, obj)
}

func (s *StorageUsecase) OpenObjectFile(ctx context.Context, obj *models.StorageObject) (*os.File, error) {
	bucket, err := s.storage.FindBucketByID(ctx, obj.BucketID)
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(s.basePath, bucket.Name, obj.Key)
	return os.Open(filePath)
}

// S3 Access Keys
func (s *StorageUsecase) ListAccessKeys(ctx context.Context, ownerID uuid.UUID) ([]models.StorageAccessKey, error) {
	return s.storage.ListAccessKeys(ctx, ownerID)
}

func (s *StorageUsecase) CreateAccessKey(ctx context.Context, ownerID uuid.UUID, input CreateAccessKeyInput) (*models.StorageAccessKey, error) {
	accessKeyID := "NVT" + strings.ToUpper(generateRandomHex(8))
	secretAccessKey := generateRandomHex(24)

	perms := input.Permissions
	if perms == "" {
		perms = "read_write"
	}
	scope := input.BucketScope
	if scope == "" {
		scope = "*"
	}

	key := models.StorageAccessKey{
		OwnerID:         ownerID,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		Description:     input.Description,
		Permissions:     perms,
		BucketScope:     scope,
		Status:          "active",
	}

	if err := s.storage.CreateAccessKey(context.WithoutCancel(ctx), &key); err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *StorageUsecase) RevokeAccessKey(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	return s.storage.RevokeAccessKey(context.WithoutCancel(ctx), id, ownerID)
}

// Shared Links

// CreateSharedLink publishes an object of the caller under a random token. A
// missing object and an object in another tenant's bucket are both reported
// as NOT_FOUND "Object not found".
func (s *StorageUsecase) CreateSharedLink(ctx context.Context, ownerID uuid.UUID, input CreateSharedLinkInput) (*models.StorageSharedLink, error) {
	ctx = context.WithoutCancel(ctx)

	obj, err := s.storage.FindObject(ctx, input.ObjectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errObjectNotFound
		}
		return nil, fmt.Errorf("object not found: %w", err)
	}
	if _, err := s.storage.FindBucket(ctx, obj.BucketID, ownerID); err != nil {
		return nil, errObjectNotFound
	}

	token := generateRandomHex(16)
	shareURL := fmt.Sprintf("/api/v1/storage/shared/%s", token)

	var expiresAt *time.Time
	if input.ExpiresIn != "" {
		d, err := time.ParseDuration(input.ExpiresIn)
		if err == nil {
			t := time.Now().Add(d)
			expiresAt = &t
		} else if strings.HasSuffix(input.ExpiresIn, "d") {
			days := 7
			fmt.Sscanf(input.ExpiresIn, "%dd", &days)
			t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &t
		}
	}

	hasPassword := false
	hashedPassword := ""
	if input.Password != "" {
		bytes, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashedPassword = string(bytes)
		hasPassword = true
	}

	link := models.StorageSharedLink{
		OwnerID:      ownerID,
		ObjectID:     input.ObjectID,
		ShareToken:   token,
		ShareURL:     shareURL,
		AccessType:   input.AccessType,
		Password:     hashedPassword,
		HasPassword:  hasPassword,
		ExpiresAt:    expiresAt,
		MaxDownloads: input.MaxDownloads,
	}

	if err := s.storage.CreateSharedLink(ctx, &link); err != nil {
		return nil, err
	}
	link.Object = obj
	return &link, nil
}

// GetSharedLink resolves a public token to a link that has neither expired
// nor reached its download limit.
func (s *StorageUsecase) GetSharedLink(ctx context.Context, token string) (*models.StorageSharedLink, error) {
	link, err := s.storage.FindSharedLinkByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("shared link has expired")
	}
	if link.MaxDownloads != nil && link.Downloads >= *link.MaxDownloads {
		return nil, fmt.Errorf("maximum download limit reached")
	}
	return link, nil
}

// OpenSharedLink resolves a public token for download, checks the link
// password when one is set and counts the download. The caller closes the
// returned reader.
func (s *StorageUsecase) OpenSharedLink(ctx context.Context, token, password string) (*models.StorageSharedLink, io.ReadCloser, error) {
	ctx = context.WithoutCancel(ctx)

	link, err := s.GetSharedLink(ctx, token)
	if err != nil {
		return nil, nil, newError(KindNotFound, "NOT_FOUND", err.Error())
	}

	if link.HasPassword {
		if password == "" {
			return nil, nil, errSharedPasswordNeeded
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.Password), []byte(password)); err != nil {
			return nil, nil, errSharedPasswordWrong
		}
	}

	file, err := s.OpenObjectFile(ctx, link.Object)
	if err != nil {
		return nil, nil, errSharedFileMissing
	}

	_ = s.storage.IncrementSharedLinkDownloads(ctx, link.ID)
	return link, file, nil
}

func (s *StorageUsecase) ListSharedLinks(ctx context.Context, ownerID uuid.UUID) ([]models.StorageSharedLink, error) {
	return s.storage.ListSharedLinks(ctx, ownerID)
}

func (s *StorageUsecase) DeleteSharedLink(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	return s.storage.DeleteSharedLink(context.WithoutCancel(ctx), id, ownerID)
}

func generateRandomHex(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// FindOwnedObject returns an object in one of ownerID's live buckets. An
// object of another tenant is reported as repository.ErrNotFound, exactly
// like a missing one.
func (s *StorageUsecase) FindOwnedObject(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageObject, error) {
	obj, err := s.storage.FindObject(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.storage.FindBucket(ctx, obj.BucketID, ownerID); err != nil {
		return nil, repository.ErrNotFound
	}
	return obj, nil
}
