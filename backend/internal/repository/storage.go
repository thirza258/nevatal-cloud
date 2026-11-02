package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// StorageRepository persists object storage: buckets, objects, S3 access
// keys and public shared links. Buckets are soft-deleted through their own
// deleted_at column.
type StorageRepository interface {
	// Buckets
	CountActiveBuckets(ctx context.Context, ownerID uuid.UUID) (int64, error)
	// SumObjects counts the objects in the owner's live buckets and totals
	// their size.
	SumObjects(ctx context.Context, ownerID uuid.UUID) (count, sizeBytes int64, err error)
	// ListBuckets returns the owner's live buckets, newest first.
	ListBuckets(ctx context.Context, ownerID uuid.UUID) ([]models.StorageBucket, error)
	// FindBucket returns a live bucket of the owner.
	FindBucket(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageBucket, error)
	// FindBucketByID returns a bucket of any owner, deleted or not.
	FindBucketByID(ctx context.Context, id uuid.UUID) (*models.StorageBucket, error)
	// FindBucketByName returns the live bucket with the given name.
	FindBucketByName(ctx context.Context, name string) (*models.StorageBucket, error)
	CreateBucket(ctx context.Context, bucket *models.StorageBucket) error
	SaveBucket(ctx context.Context, bucket *models.StorageBucket) error
	// UpdateBucketSize sets total_size_bytes on bucket.
	UpdateBucketSize(ctx context.Context, bucket *models.StorageBucket, sizeBytes int64) error
	// UpdateBucketTotals sets total_objects and total_size_bytes on bucket.
	UpdateBucketTotals(ctx context.Context, bucket *models.StorageBucket, objects, sizeBytes int64) error

	// Objects
	CountObjects(ctx context.Context, bucketID uuid.UUID) (int64, error)
	// ListObjects returns a bucket's objects (folders first, then by name),
	// optionally limited to a folder prefix.
	ListObjects(ctx context.Context, bucketID uuid.UUID, prefix string) ([]models.StorageObject, error)
	FindObject(ctx context.Context, id uuid.UUID) (*models.StorageObject, error)
	FindObjectByKey(ctx context.Context, bucketID uuid.UUID, key string) (*models.StorageObject, error)
	CreateObject(ctx context.Context, obj *models.StorageObject) error
	SaveObject(ctx context.Context, obj *models.StorageObject) error
	DeleteObject(ctx context.Context, obj *models.StorageObject) error

	// Access keys
	CountActiveAccessKeys(ctx context.Context, ownerID uuid.UUID) (int64, error)
	// ListAccessKeys returns the owner's keys, newest first.
	ListAccessKeys(ctx context.Context, ownerID uuid.UUID) ([]models.StorageAccessKey, error)
	CreateAccessKey(ctx context.Context, key *models.StorageAccessKey) error
	// RevokeAccessKey marks the owner's key revoked.
	RevokeAccessKey(ctx context.Context, id, ownerID uuid.UUID) error

	// Shared links
	CreateSharedLink(ctx context.Context, link *models.StorageSharedLink) error
	// FindSharedLinkByToken returns a link with its Object preloaded.
	FindSharedLinkByToken(ctx context.Context, token string) (*models.StorageSharedLink, error)
	IncrementSharedLinkDownloads(ctx context.Context, id uuid.UUID) error
	// ListSharedLinks returns the owner's links (newest first) with Object
	// preloaded.
	ListSharedLinks(ctx context.Context, ownerID uuid.UUID) ([]models.StorageSharedLink, error)
	// DeleteSharedLink deletes the owner's link.
	DeleteSharedLink(ctx context.Context, id, ownerID uuid.UUID) error
}

type storageRepository struct{ base }

var _ StorageRepository = (*storageRepository)(nil)

func NewStorageRepository(db *gorm.DB) StorageRepository {
	return &storageRepository{base{db}}
}

func (r *storageRepository) CountActiveBuckets(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.StorageBucket{}).Where("owner_id = ? AND deleted_at IS NULL", ownerID).Count(&n).Error
	return n, err
}

func (r *storageRepository) SumObjects(ctx context.Context, ownerID uuid.UUID) (int64, int64, error) {
	type SizeResult struct {
		Count int64
		Size  int64
	}
	var res SizeResult
	err := r.conn(ctx).Model(&models.StorageObject{}).
		Joins("JOIN storage_buckets ON storage_buckets.id = storage_objects.bucket_id").
		Where("storage_buckets.owner_id = ? AND storage_buckets.deleted_at IS NULL", ownerID).
		Select("COUNT(storage_objects.id) as count, COALESCE(SUM(storage_objects.size_bytes), 0) as size").
		Scan(&res).Error
	return res.Count, res.Size, err
}

func (r *storageRepository) ListBuckets(ctx context.Context, ownerID uuid.UUID) ([]models.StorageBucket, error) {
	var buckets []models.StorageBucket
	err := r.conn(ctx).Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Order("created_at DESC").
		Find(&buckets).Error
	return buckets, err
}

func (r *storageRepository) FindBucket(ctx context.Context, id, ownerID uuid.UUID) (*models.StorageBucket, error) {
	var bucket models.StorageBucket
	if err := r.conn(ctx).Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).First(&bucket).Error; err != nil {
		return nil, mapErr(err)
	}
	return &bucket, nil
}

func (r *storageRepository) FindBucketByID(ctx context.Context, id uuid.UUID) (*models.StorageBucket, error) {
	var bucket models.StorageBucket
	if err := r.conn(ctx).First(&bucket, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &bucket, nil
}

func (r *storageRepository) FindBucketByName(ctx context.Context, name string) (*models.StorageBucket, error) {
	var bucket models.StorageBucket
	if err := r.conn(ctx).Where("name = ? AND deleted_at IS NULL", name).First(&bucket).Error; err != nil {
		return nil, mapErr(err)
	}
	return &bucket, nil
}

func (r *storageRepository) CreateBucket(ctx context.Context, bucket *models.StorageBucket) error {
	return r.conn(ctx).Create(bucket).Error
}

func (r *storageRepository) SaveBucket(ctx context.Context, bucket *models.StorageBucket) error {
	return r.conn(ctx).Save(bucket).Error
}

func (r *storageRepository) UpdateBucketSize(ctx context.Context, bucket *models.StorageBucket, sizeBytes int64) error {
	return r.conn(ctx).Model(bucket).Update("total_size_bytes", sizeBytes).Error
}

func (r *storageRepository) UpdateBucketTotals(ctx context.Context, bucket *models.StorageBucket, objects, sizeBytes int64) error {
	return r.conn(ctx).Model(bucket).Updates(map[string]interface{}{
		"total_objects":    objects,
		"total_size_bytes": sizeBytes,
	}).Error
}

func (r *storageRepository) CountObjects(ctx context.Context, bucketID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.StorageObject{}).Where("bucket_id = ?", bucketID).Count(&n).Error
	return n, err
}

func (r *storageRepository) ListObjects(ctx context.Context, bucketID uuid.UUID, prefix string) ([]models.StorageObject, error) {
	var objects []models.StorageObject
	q := r.conn(ctx).Where("bucket_id = ?", bucketID)
	if prefix != "" {
		q = q.Where("folder_prefix = ? OR key LIKE ?", prefix, prefix+"%")
	}
	err := q.Order("is_folder DESC, filename ASC").Find(&objects).Error
	return objects, err
}

func (r *storageRepository) FindObject(ctx context.Context, id uuid.UUID) (*models.StorageObject, error) {
	var obj models.StorageObject
	if err := r.conn(ctx).First(&obj, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &obj, nil
}

func (r *storageRepository) FindObjectByKey(ctx context.Context, bucketID uuid.UUID, key string) (*models.StorageObject, error) {
	var obj models.StorageObject
	if err := r.conn(ctx).Where("bucket_id = ? AND key = ?", bucketID, key).First(&obj).Error; err != nil {
		return nil, mapErr(err)
	}
	return &obj, nil
}

func (r *storageRepository) CreateObject(ctx context.Context, obj *models.StorageObject) error {
	return r.conn(ctx).Create(obj).Error
}

func (r *storageRepository) SaveObject(ctx context.Context, obj *models.StorageObject) error {
	return r.conn(ctx).Save(obj).Error
}

func (r *storageRepository) DeleteObject(ctx context.Context, obj *models.StorageObject) error {
	return r.conn(ctx).Delete(obj).Error
}

func (r *storageRepository) CountActiveAccessKeys(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.StorageAccessKey{}).Where("owner_id = ? AND status = 'active'", ownerID).Count(&n).Error
	return n, err
}

func (r *storageRepository) ListAccessKeys(ctx context.Context, ownerID uuid.UUID) ([]models.StorageAccessKey, error) {
	var keys []models.StorageAccessKey
	err := r.conn(ctx).Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *storageRepository) CreateAccessKey(ctx context.Context, key *models.StorageAccessKey) error {
	return r.conn(ctx).Create(key).Error
}

func (r *storageRepository) RevokeAccessKey(ctx context.Context, id, ownerID uuid.UUID) error {
	return r.conn(ctx).Model(&models.StorageAccessKey{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("status", "revoked").Error
}

func (r *storageRepository) CreateSharedLink(ctx context.Context, link *models.StorageSharedLink) error {
	return r.conn(ctx).Create(link).Error
}

func (r *storageRepository) FindSharedLinkByToken(ctx context.Context, token string) (*models.StorageSharedLink, error) {
	var link models.StorageSharedLink
	if err := r.conn(ctx).Preload("Object").Where("share_token = ?", token).First(&link).Error; err != nil {
		return nil, mapErr(err)
	}
	return &link, nil
}

func (r *storageRepository) IncrementSharedLinkDownloads(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Model(&models.StorageSharedLink{}).Where("id = ?", id).UpdateColumn("downloads", gorm.Expr("downloads + 1")).Error
}

func (r *storageRepository) ListSharedLinks(ctx context.Context, ownerID uuid.UUID) ([]models.StorageSharedLink, error) {
	var links []models.StorageSharedLink
	err := r.conn(ctx).Preload("Object").Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&links).Error
	return links, err
}

func (r *storageRepository) DeleteSharedLink(ctx context.Context, id, ownerID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.StorageSharedLink{}).Error
}
