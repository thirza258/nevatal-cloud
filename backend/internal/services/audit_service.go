package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Log(actorID *uuid.UUID, action, resourceType string, resourceID *uuid.UUID, result, ipAddress string, metadata interface{}) {
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

	_ = s.db.Create(&entry)
}

func (s *AuditService) ListRecentActivity(userID *uuid.UUID, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}

	query := s.db.Model(&models.AuditLog{}).Preload("Actor").Order("created_at desc").Limit(limit)
	if userID != nil {
		query = query.Where("actor_user_id = ?", *userID)
	}

	var logs []models.AuditLog
	err := query.Find(&logs).Error
	return logs, err
}
