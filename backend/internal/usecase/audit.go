package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

type AuditUsecase struct {
	audits repository.AuditRepository
}

func NewAuditUsecase(audits repository.AuditRepository) *AuditUsecase {
	return &AuditUsecase{audits: audits}
}

// Log records an audit entry. Failures are ignored so auditing never breaks
// the audited action. The write is detached from ctx cancellation: an entry is
// still recorded when the client disconnects mid-request.
func (s *AuditUsecase) Log(ctx context.Context, actorID *uuid.UUID, action, resourceType string, resourceID *uuid.UUID, result, ipAddress string, metadata interface{}) {
	metaJSON := "{}"
	if metadata != nil {
		if b, err := json.Marshal(metadata); err == nil {
			metaJSON = string(b)
		}
	}

	entry := models.AuditLog{
		ActorUserID:  actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		IPAddress:    &ipAddress,
		Metadata:     metaJSON,
		CreatedAt:    time.Now(),
	}

	_ = s.audits.Create(context.WithoutCancel(ctx), &entry)
}

// ListRecentActivity returns the newest entries (default 20), optionally only
// those performed by userID.
func (s *AuditUsecase) ListRecentActivity(ctx context.Context, userID *uuid.UUID, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.audits.ListRecent(ctx, userID, limit)
}
