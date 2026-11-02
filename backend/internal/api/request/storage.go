package request

import (
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

// CreateBucketInput is the body of POST /storage/buckets.
type CreateBucketInput struct {
	Name              string `json:"name" binding:"required"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	VersioningEnabled bool   `json:"versioning_enabled"`
	Region            string `json:"region"`
}

func (r CreateBucketInput) ToUsecase() usecase.CreateBucketInput {
	return usecase.CreateBucketInput(r)
}

// UpdateBucketInput is the body of PUT /storage/buckets/:id.
type UpdateBucketInput struct {
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	VersioningEnabled *bool  `json:"versioning_enabled"`
}

func (r UpdateBucketInput) ToUsecase() usecase.UpdateBucketInput {
	return usecase.UpdateBucketInput(r)
}

// CreateFolderRequest is the body of POST /storage/buckets/:id/folders.
type CreateFolderRequest = struct {
	FolderPrefix string `json:"folder_prefix"`
	FolderName   string `json:"folder_name" binding:"required"`
}

// CreateAccessKeyInput is the body of POST /storage/access-keys.
type CreateAccessKeyInput struct {
	Description string `json:"description"`
	Permissions string `json:"permissions"`
	BucketScope string `json:"bucket_scope"`
}

func (r CreateAccessKeyInput) ToUsecase() usecase.CreateAccessKeyInput {
	return usecase.CreateAccessKeyInput(r)
}

// CreateSharedLinkInput is the body of POST /storage/shared-links.
type CreateSharedLinkInput struct {
	ObjectID     uuid.UUID `json:"object_id" binding:"required"`
	AccessType   string    `json:"access_type"`
	Password     string    `json:"password"`
	ExpiresIn    string    `json:"expires_in"` // e.g. "1h", "24h", "7d", "30d"
	MaxDownloads *int64    `json:"max_downloads"`
}

func (r CreateSharedLinkInput) ToUsecase() usecase.CreateSharedLinkInput {
	return usecase.CreateSharedLinkInput(r)
}
