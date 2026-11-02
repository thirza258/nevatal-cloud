package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
)

// SharedLinkInfo is the public view of a shared link
// (GET /storage/shared/:token).
type SharedLinkInfo struct {
	AccessType  string     `json:"access_type"`
	ContentType string     `json:"content_type"`
	ExpiresAt   *time.Time `json:"expires_at"`
	Filename    string     `json:"filename"`
	HasPassword bool       `json:"has_password"`
	ID          uuid.UUID  `json:"id"`
	SizeBytes   int64      `json:"size_bytes"`
}

func NewSharedLinkInfo(link *models.StorageSharedLink) SharedLinkInfo {
	return SharedLinkInfo{
		AccessType:  link.AccessType,
		ContentType: link.Object.ContentType,
		ExpiresAt:   link.ExpiresAt,
		Filename:    link.Object.Filename,
		HasPassword: link.HasPassword,
		ID:          link.ID,
		SizeBytes:   link.Object.SizeBytes,
	}
}
