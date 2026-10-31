package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/nevatal/cloud/internal/models"
)

type StorageService struct {
	db       *gorm.DB
	basePath string
}

func NewStorageService(db *gorm.DB, basePath string) *StorageService {
	if basePath == "" {
		basePath = "/data/storage"
	}
	os.MkdirAll(basePath, 0755)
	return &StorageService{
		db:       db,
		basePath: basePath,
	}
}

type CreateBucketInput struct {
	Name              string `json:"name" binding:"required"`
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
	ObjectID     uuid.UUID `json:"object_id" binding:"required"`
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

func (s *StorageService) GetOverview(ownerID uuid.UUID) (*StorageOverviewResponse, error) {
	var totalBuckets int64
	var totalObjects int64
	var totalSizeBytes int64
	var totalKeys int64

	s.db.Model(&models.StorageBucket{}).Where("owner_id = ? AND deleted_at IS NULL", ownerID).Count(&totalBuckets)

	type SizeResult struct {
		Count int64
		Size  int64
	}
	var res SizeResult
	s.db.Model(&models.StorageObject{}).
		Joins("JOIN storage_buckets ON storage_buckets.id = storage_objects.bucket_id").
		Where("storage_buckets.owner_id = ? AND storage_buckets.deleted_at IS NULL", ownerID).
		Select("COUNT(storage_objects.id) as count, COALESCE(SUM(storage_objects.size_bytes), 0) as size").
		Scan(&res)
	totalObjects = res.Count
	totalSizeBytes = res.Size

	s.db.Model(&models.StorageAccessKey{}).Where("owner_id = ? AND status = 'active'", ownerID).Count(&totalKeys)

	return &StorageOverviewResponse{
		TotalBuckets:    totalBuckets,
		TotalObjects:    totalObjects,
		TotalSizeBytes:  totalSizeBytes,
		TotalAccessKeys: totalKeys,
	}, nil
}

func (s *StorageService) ListBuckets(ownerID uuid.UUID) ([]models.StorageBucket, error) {
	var buckets []models.StorageBucket
	err := s.db.Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at DESC").
		Find(&buckets).Error
	return buckets, err
}

func (s *StorageService) GetBucket(id uuid.UUID, ownerID uuid.UUID) (*models.StorageBucket, error) {
	var bucket models.StorageBucket
	err := s.db.Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (s *StorageService) GetBucketByName(name string) (*models.StorageBucket, error) {
	var bucket models.StorageBucket
	err := s.db.Where("name = ? AND deleted_at IS NULL", name).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (s *StorageService) CreateBucket(ownerID uuid.UUID, input CreateBucketInput) (*models.StorageBucket, error) {
	// Sanitize name according to S3 rules (lowercase, alphanumeric, hyphens)
	name := strings.ToLower(strings.TrimSpace(input.Name))
	if len(name) < 3 || len(name) > 63 {
		return nil, fmt.Errorf("bucket name must be between 3 and 63 characters")
	}

	var existing models.StorageBucket
	if err := s.db.Where("name = ? AND deleted_at IS NULL", name).First(&existing).Error; err == nil {
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

	if err := s.db.Create(&bucket).Error; err != nil {
		return nil, err
	}

	bucketDir := filepath.Join(s.basePath, bucket.Name)
	_ = os.MkdirAll(bucketDir, 0755)

	return &bucket, nil
}

func (s *StorageService) UpdateBucket(id uuid.UUID, ownerID uuid.UUID, input UpdateBucketInput) (*models.StorageBucket, error) {
	bucket, err := s.GetBucket(id, ownerID)
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

	if err := s.db.Save(bucket).Error; err != nil {
		return nil, err
	}
	return bucket, nil
}

func (s *StorageService) DeleteBucket(id uuid.UUID, ownerID uuid.UUID) error {
	bucket, err := s.GetBucket(id, ownerID)
	if err != nil {
		return err
	}

	var count int64
	s.db.Model(&models.StorageObject{}).Where("bucket_id = ?", bucket.ID).Count(&count)
	if count > 0 {
		return fmt.Errorf("bucket is not empty (%d objects found). Delete objects first", count)
	}

	now := time.Now()
	bucket.DeletedAt = &now
	if err := s.db.Save(bucket).Error; err != nil {
		return err
	}

	bucketDir := filepath.Join(s.basePath, bucket.Name)
	_ = os.RemoveAll(bucketDir)

	return nil
}

func (s *StorageService) ListObjects(bucketID uuid.UUID, prefix string) ([]models.StorageObject, error) {
	var objects []models.StorageObject
	q := s.db.Where("bucket_id = ?", bucketID)
	if prefix != "" {
		q = q.Where("folder_prefix = ? OR key LIKE ?", prefix, prefix+"%")
	}
	err := q.Order("is_folder DESC, filename ASC").Find(&objects).Error
	return objects, err
}

func (s *StorageService) GetObject(id uuid.UUID) (*models.StorageObject, error) {
	var obj models.StorageObject
	if err := s.db.First(&obj, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &obj, nil
}

func (s *StorageService) SaveObject(bucket *models.StorageBucket, folderPrefix, filename, contentType string, reader io.Reader) (*models.StorageObject, error) {
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

	var existing models.StorageObject
	err = s.db.Where("bucket_id = ? AND key = ?", bucket.ID, key).First(&existing).Error
	if err == nil {
		// Update existing
		oldSize := existing.SizeBytes
		existing.SizeBytes = sizeBytes
		existing.ContentType = contentType
		existing.ETag = etag
		existing.UpdatedAt = time.Now()
		if err := s.db.Save(&existing).Error; err != nil {
			return nil, err
		}

		bucket.TotalSizeBytes += (sizeBytes - oldSize)
		s.db.Model(bucket).Update("total_size_bytes", bucket.TotalSizeBytes)
		return &existing, nil
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

	if err := s.db.Create(&obj).Error; err != nil {
		return nil, err
	}

	// Update bucket totals
	bucket.TotalObjects++
	bucket.TotalSizeBytes += sizeBytes
	s.db.Model(bucket).Updates(map[string]interface{}{
		"total_objects":    bucket.TotalObjects,
		"total_size_bytes": bucket.TotalSizeBytes,
	})

	return &obj, nil
}

func (s *StorageService) CreateFolder(bucket *models.StorageBucket, folderPrefix, folderName string) (*models.StorageObject, error) {
	cleanPrefix := strings.Trim(folderPrefix, "/")
	cleanName := strings.Trim(folderName, "/")
	var key string
	if cleanPrefix != "" {
		key = cleanPrefix + "/" + cleanName + "/"
	} else {
		key = cleanName + "/"
	}

	var existing models.StorageObject
	if err := s.db.Where("bucket_id = ? AND key = ?", bucket.ID, key).First(&existing).Error; err == nil {
		return &existing, nil
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

	if err := s.db.Create(&obj).Error; err != nil {
		return nil, err
	}

	diskDir := filepath.Join(s.basePath, bucket.Name, cleanPrefix, cleanName)
	_ = os.MkdirAll(diskDir, 0755)

	return &obj, nil
}

func (s *StorageService) DeleteObject(id uuid.UUID) error {
	var obj models.StorageObject
	if err := s.db.First(&obj, "id = ?", id).Error; err != nil {
		return err
	}

	var bucket models.StorageBucket
	if err := s.db.First(&bucket, "id = ?", obj.BucketID).Error; err == nil {
		if !obj.IsFolder {
			bucket.TotalObjects--
			bucket.TotalSizeBytes -= obj.SizeBytes
			if bucket.TotalSizeBytes < 0 {
				bucket.TotalSizeBytes = 0
			}
			s.db.Model(&bucket).Updates(map[string]interface{}{
				"total_objects":    bucket.TotalObjects,
				"total_size_bytes": bucket.TotalSizeBytes,
			})
		}
		filePath := filepath.Join(s.basePath, bucket.Name, obj.Key)
		_ = os.RemoveAll(filePath)
	}

	return s.db.Delete(&obj).Error
}

func (s *StorageService) OpenObjectFile(obj *models.StorageObject) (*os.File, error) {
	var bucket models.StorageBucket
	if err := s.db.First(&bucket, "id = ?", obj.BucketID).Error; err != nil {
		return nil, err
	}
	filePath := filepath.Join(s.basePath, bucket.Name, obj.Key)
	return os.Open(filePath)
}

// S3 Access Keys
func (s *StorageService) ListAccessKeys(ownerID uuid.UUID) ([]models.StorageAccessKey, error) {
	var keys []models.StorageAccessKey
	err := s.db.Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (s *StorageService) CreateAccessKey(ownerID uuid.UUID, input CreateAccessKeyInput) (*models.StorageAccessKey, error) {
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

	if err := s.db.Create(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (s *StorageService) RevokeAccessKey(id uuid.UUID, ownerID uuid.UUID) error {
	return s.db.Model(&models.StorageAccessKey{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("status", "revoked").Error
}

// Shared Links
func (s *StorageService) CreateSharedLink(ownerID uuid.UUID, input CreateSharedLinkInput) (*models.StorageSharedLink, error) {
	var obj models.StorageObject
	if err := s.db.First(&obj, "id = ?", input.ObjectID).Error; err != nil {
		return nil, fmt.Errorf("object not found: %w", err)
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

	if err := s.db.Create(&link).Error; err != nil {
		return nil, err
	}
	link.Object = &obj
	return &link, nil
}

func (s *StorageService) GetSharedLink(token string) (*models.StorageSharedLink, error) {
	var link models.StorageSharedLink
	err := s.db.Preload("Object").Where("share_token = ?", token).First(&link).Error
	if err != nil {
		return nil, err
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("shared link has expired")
	}
	if link.MaxDownloads != nil && link.Downloads >= *link.MaxDownloads {
		return nil, fmt.Errorf("maximum download limit reached")
	}
	return &link, nil
}

func (s *StorageService) IncrementSharedDownload(id uuid.UUID) {
	s.db.Model(&models.StorageSharedLink{}).Where("id = ?", id).UpdateColumn("downloads", gorm.Expr("downloads + 1"))
}

func (s *StorageService) ListSharedLinks(ownerID uuid.UUID) ([]models.StorageSharedLink, error) {
	var links []models.StorageSharedLink
	err := s.db.Preload("Object").Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&links).Error
	return links, err
}

func (s *StorageService) DeleteSharedLink(id uuid.UUID, ownerID uuid.UUID) error {
	return s.db.Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.StorageSharedLink{}).Error
}

func generateRandomHex(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
