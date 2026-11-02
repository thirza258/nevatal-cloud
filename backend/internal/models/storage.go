package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StorageBucket struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name              string     `gorm:"type:varchar(63);not null;unique" json:"name"` // S3 naming restrictions
	Description       string     `gorm:"type:varchar(255)" json:"description"`
	Visibility        string     `gorm:"type:varchar(32);not null;default:'private'" json:"visibility"` // private, public
	VersioningEnabled bool       `gorm:"not null;default:false" json:"versioning_enabled"`
	Region            string     `gorm:"type:varchar(50);not null;default:'id-central-1'" json:"region"`
	TotalObjects      int64      `gorm:"default:0" json:"total_objects"`
	TotalSizeBytes    int64      `gorm:"default:0" json:"total_size_bytes"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`

	Owner   *User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Objects []StorageObject `gorm:"foreignKey:BucketID" json:"objects,omitempty"`
}

func (b *StorageBucket) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (StorageBucket) TableName() string {
	return "storage_buckets"
}

type StorageObject struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	BucketID     uuid.UUID `gorm:"type:uuid;not null;index" json:"bucket_id"`
	Key          string    `gorm:"type:varchar(1024);not null;index" json:"key"` // e.g. "photos/vacation.jpg"
	Filename     string    `gorm:"type:varchar(255);not null" json:"filename"`
	FolderPrefix string    `gorm:"type:varchar(512);default:''" json:"folder_prefix"`
	ContentType  string    `gorm:"type:varchar(150);not null;default:'application/octet-stream'" json:"content_type"`
	SizeBytes    int64     `gorm:"not null" json:"size_bytes"`
	ETag         string    `gorm:"column:etag;type:varchar(64)" json:"etag"`
	IsFolder     bool      `gorm:"not null;default:false" json:"is_folder"`
	PublicURL    string    `gorm:"type:varchar(512)" json:"public_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (o *StorageObject) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (StorageObject) TableName() string {
	return "storage_objects"
}

type StorageAccessKey struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	AccessKeyID     string     `gorm:"type:varchar(64);not null;unique" json:"access_key_id"`
	SecretAccessKey string     `gorm:"type:varchar(128);not null" json:"secret_access_key,omitempty"` // only shown on creation
	Description     string     `gorm:"type:varchar(255)" json:"description"`
	Permissions     string     `gorm:"type:varchar(50);not null;default:'read_write'" json:"permissions"` // read_only, read_write, admin
	BucketScope     string     `gorm:"type:varchar(255);default:'*'" json:"bucket_scope"`                 // specific bucket or '*'
	Status          string     `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (k *StorageAccessKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

func (StorageAccessKey) TableName() string {
	return "storage_access_keys"
}

type StorageSharedLink struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	ObjectID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"object_id"`
	ShareToken   string     `gorm:"type:varchar(64);unique;not null" json:"share_token"`
	ShareURL     string     `gorm:"type:varchar(512);not null" json:"share_url"`
	AccessType   string     `gorm:"type:varchar(32);default:'download'" json:"access_type"` // view, download
	Password     string     `gorm:"type:varchar(255)" json:"-"`
	HasPassword  bool       `gorm:"default:false" json:"has_password"`
	ExpiresAt    *time.Time `json:"expires_at"`
	Downloads    int64      `gorm:"default:0" json:"downloads"`
	MaxDownloads *int64     `json:"max_downloads,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`

	Object *StorageObject `gorm:"foreignKey:ObjectID" json:"object,omitempty"`
}

func (s *StorageSharedLink) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (StorageSharedLink) TableName() string {
	return "storage_shared_links"
}
