package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AIAssistantRepository persists the AI assistant chat history
// (ai_assistant_messages).
type AIAssistantRepository interface {
	// ListHistory returns the user's oldest messages first, at most limit.
	ListHistory(ctx context.Context, userID uuid.UUID, limit int) ([]models.AIAssistantMessage, error)
	// ListRecent returns the user's newest messages first, at most limit.
	ListRecent(ctx context.Context, userID uuid.UUID, limit int) ([]models.AIAssistantMessage, error)
	Create(ctx context.Context, msg *models.AIAssistantMessage) error
	// MarkActionExecuted flags the proposal in one of the user's messages as
	// executed.
	MarkActionExecuted(ctx context.Context, messageID, userID uuid.UUID) error
	DeleteHistory(ctx context.Context, userID uuid.UUID) error
}

type aiAssistantRepository struct{ base }

var _ AIAssistantRepository = (*aiAssistantRepository)(nil)

func NewAIAssistantRepository(db *gorm.DB) AIAssistantRepository {
	return &aiAssistantRepository{base{db}}
}

func (r *aiAssistantRepository) ListHistory(ctx context.Context, userID uuid.UUID, limit int) ([]models.AIAssistantMessage, error) {
	var messages []models.AIAssistantMessage
	err := r.conn(ctx).Where("user_id = ?", userID).
		Order("created_at asc").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func (r *aiAssistantRepository) ListRecent(ctx context.Context, userID uuid.UUID, limit int) ([]models.AIAssistantMessage, error) {
	var history []models.AIAssistantMessage
	err := r.conn(ctx).Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Find(&history).Error
	return history, err
}

func (r *aiAssistantRepository) Create(ctx context.Context, msg *models.AIAssistantMessage) error {
	return r.conn(ctx).Create(msg).Error
}

func (r *aiAssistantRepository) MarkActionExecuted(ctx context.Context, messageID, userID uuid.UUID) error {
	return r.conn(ctx).Model(&models.AIAssistantMessage{}).
		Where("id = ? AND user_id = ?", messageID, userID).
		Update("action_executed", true).Error
}

func (r *aiAssistantRepository) DeleteHistory(ctx context.Context, userID uuid.UUID) error {
	return r.conn(ctx).Where("user_id = ?", userID).Delete(&models.AIAssistantMessage{}).Error
}
