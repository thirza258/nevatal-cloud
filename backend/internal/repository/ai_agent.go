package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AIAgentRepository persists AI cloud agent conversations: their messages
// and workflow state, the resource plans the agent proposes, the plan
// resources and the user approvals.
type AIAgentRepository interface {
	// Conversations
	CreateConversation(ctx context.Context, conv *models.AIConversation) error
	// ReloadConversation refreshes conv with its messages and state.
	ReloadConversation(ctx context.Context, conv *models.AIConversation) error
	// ListConversations returns the user's conversations, most recently
	// updated first.
	ListConversations(ctx context.Context, userID uuid.UUID) ([]models.AIConversation, error)
	// FindConversation returns one of the user's conversations.
	FindConversation(ctx context.Context, convID, userID uuid.UUID) (*models.AIConversation, error)
	// FindConversationDetails returns one of the user's conversations with
	// messages (oldest first), plans with resources, and state.
	FindConversationDetails(ctx context.Context, convID, userID uuid.UUID) (*models.AIConversation, error)
	SaveConversation(ctx context.Context, conv *models.AIConversation) error
	// SetConversationTitle sets the title of conv.
	SetConversationTitle(ctx context.Context, conv *models.AIConversation, title string) error
	// SetConversationState sets the workflow state of a conversation.
	SetConversationState(ctx context.Context, convID uuid.UUID, state models.WorkflowState) error
	// DeleteConversation deletes one of the user's conversations.
	DeleteConversation(ctx context.Context, convID, userID uuid.UUID) error
	// FindConversationByTitle returns the user's newest conversation with
	// this exact title.
	FindConversationByTitle(ctx context.Context, userID uuid.UUID, title string) (*models.AIConversation, error)

	// Workflow state
	FindState(ctx context.Context, convID uuid.UUID) (*models.AIConversationState, error)
	CreateState(ctx context.Context, state *models.AIConversationState) error
	SaveState(ctx context.Context, state *models.AIConversationState) error

	// Messages
	CreateMessage(ctx context.Context, msg *models.AIMessage) error
	// ListMessages returns a conversation's messages, oldest first.
	ListMessages(ctx context.Context, convID uuid.UUID) ([]models.AIMessage, error)

	// Plans
	// ListPlans returns the user's plans of a conversation (newest version
	// first) with resources.
	ListPlans(ctx context.Context, convID, userID uuid.UUID) ([]models.AIResourcePlan, error)
	// FindPlanDetails returns one of the user's plans with resources and
	// approvals.
	FindPlanDetails(ctx context.Context, planID, userID uuid.UUID) (*models.AIResourcePlan, error)
	// FindConversationPlan returns one of the user's plans of a conversation.
	FindConversationPlan(ctx context.Context, planID, userID, convID uuid.UUID) (*models.AIResourcePlan, error)
	// FindConversationPlanWithResources is FindConversationPlan with the
	// plan resources preloaded.
	FindConversationPlanWithResources(ctx context.Context, planID, userID, convID uuid.UUID) (*models.AIResourcePlan, error)
	CreatePlan(ctx context.Context, plan *models.AIResourcePlan) error
	SavePlan(ctx context.Context, plan *models.AIResourcePlan) error
	CreatePlanResource(ctx context.Context, res *models.AIPlanResource) error
	SavePlanResource(ctx context.Context, res *models.AIPlanResource) error

	// Approvals
	CreateApproval(ctx context.Context, approval *models.AIApproval) error
	// FindPendingApproval returns the user's pending approval of a plan.
	FindPendingApproval(ctx context.Context, planID, userID uuid.UUID) (*models.AIApproval, error)
	SaveApproval(ctx context.Context, approval *models.AIApproval) error
	// CancelApprovals marks the user's live (approved or pending) approvals
	// of a plan cancelled.
	CancelApprovals(ctx context.Context, planID, userID uuid.UUID, at *time.Time) error
}

type aiAgentRepository struct{ base }

var _ AIAgentRepository = (*aiAgentRepository)(nil)

func NewAIAgentRepository(db *gorm.DB) AIAgentRepository {
	return &aiAgentRepository{base{db}}
}

func (r *aiAgentRepository) CreateConversation(ctx context.Context, conv *models.AIConversation) error {
	return r.conn(ctx).Create(conv).Error
}

func (r *aiAgentRepository) ReloadConversation(ctx context.Context, conv *models.AIConversation) error {
	return r.conn(ctx).Preload("Messages").Preload("StateObj").First(conv, "id = ?", conv.ID).Error
}

func (r *aiAgentRepository) ListConversations(ctx context.Context, userID uuid.UUID) ([]models.AIConversation, error) {
	var convs []models.AIConversation
	err := r.conn(ctx).Where("user_id = ?", userID).
		Order("updated_at desc").
		Find(&convs).Error
	return convs, err
}

func (r *aiAgentRepository) FindConversation(ctx context.Context, convID, userID uuid.UUID) (*models.AIConversation, error) {
	var conv models.AIConversation
	if err := r.conn(ctx).First(&conv, "id = ? AND user_id = ?", convID, userID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &conv, nil
}

func (r *aiAgentRepository) FindConversationDetails(ctx context.Context, convID, userID uuid.UUID) (*models.AIConversation, error) {
	var conv models.AIConversation
	err := r.conn(ctx).Preload("Messages", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at asc")
	}).Preload("Plans.Resources").Preload("StateObj").
		First(&conv, "id = ? AND user_id = ?", convID, userID).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &conv, nil
}

func (r *aiAgentRepository) SaveConversation(ctx context.Context, conv *models.AIConversation) error {
	return r.conn(ctx).Save(conv).Error
}

func (r *aiAgentRepository) SetConversationTitle(ctx context.Context, conv *models.AIConversation, title string) error {
	return r.conn(ctx).Model(conv).Update("title", title).Error
}

func (r *aiAgentRepository) SetConversationState(ctx context.Context, convID uuid.UUID, state models.WorkflowState) error {
	return r.conn(ctx).Model(&models.AIConversation{}).Where("id = ?", convID).Update("state", state).Error
}

func (r *aiAgentRepository) DeleteConversation(ctx context.Context, convID, userID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND user_id = ?", convID, userID).Delete(&models.AIConversation{}).Error
}

func (r *aiAgentRepository) FindConversationByTitle(ctx context.Context, userID uuid.UUID, title string) (*models.AIConversation, error) {
	var conv models.AIConversation
	if err := r.conn(ctx).Where("user_id = ? AND title = ?", userID, title).Order("created_at desc").First(&conv).Error; err != nil {
		return nil, mapErr(err)
	}
	return &conv, nil
}

func (r *aiAgentRepository) FindState(ctx context.Context, convID uuid.UUID) (*models.AIConversationState, error) {
	var state models.AIConversationState
	if err := r.conn(ctx).First(&state, "conversation_id = ?", convID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &state, nil
}

func (r *aiAgentRepository) CreateState(ctx context.Context, state *models.AIConversationState) error {
	return r.conn(ctx).Create(state).Error
}

func (r *aiAgentRepository) SaveState(ctx context.Context, state *models.AIConversationState) error {
	return r.conn(ctx).Save(state).Error
}

func (r *aiAgentRepository) CreateMessage(ctx context.Context, msg *models.AIMessage) error {
	return r.conn(ctx).Create(msg).Error
}

func (r *aiAgentRepository) ListMessages(ctx context.Context, convID uuid.UUID) ([]models.AIMessage, error) {
	var msgs []models.AIMessage
	err := r.conn(ctx).Where("conversation_id = ?", convID).Order("created_at asc").Find(&msgs).Error
	return msgs, err
}

func (r *aiAgentRepository) ListPlans(ctx context.Context, convID, userID uuid.UUID) ([]models.AIResourcePlan, error) {
	var plans []models.AIResourcePlan
	err := r.conn(ctx).Preload("Resources").
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Order("version desc").
		Find(&plans).Error
	return plans, err
}

func (r *aiAgentRepository) FindPlanDetails(ctx context.Context, planID, userID uuid.UUID) (*models.AIResourcePlan, error) {
	var plan models.AIResourcePlan
	err := r.conn(ctx).Preload("Resources").Preload("Approvals").
		First(&plan, "id = ? AND user_id = ?", planID, userID).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *aiAgentRepository) FindConversationPlan(ctx context.Context, planID, userID, convID uuid.UUID) (*models.AIResourcePlan, error) {
	var plan models.AIResourcePlan
	if err := r.conn(ctx).First(&plan, "id = ? AND user_id = ? AND conversation_id = ?", planID, userID, convID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *aiAgentRepository) FindConversationPlanWithResources(ctx context.Context, planID, userID, convID uuid.UUID) (*models.AIResourcePlan, error) {
	var plan models.AIResourcePlan
	if err := r.conn(ctx).Preload("Resources").
		First(&plan, "id = ? AND user_id = ? AND conversation_id = ?", planID, userID, convID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *aiAgentRepository) CreatePlan(ctx context.Context, plan *models.AIResourcePlan) error {
	return r.conn(ctx).Create(plan).Error
}

func (r *aiAgentRepository) SavePlan(ctx context.Context, plan *models.AIResourcePlan) error {
	return r.conn(ctx).Save(plan).Error
}

func (r *aiAgentRepository) CreatePlanResource(ctx context.Context, res *models.AIPlanResource) error {
	return r.conn(ctx).Create(res).Error
}

func (r *aiAgentRepository) SavePlanResource(ctx context.Context, res *models.AIPlanResource) error {
	return r.conn(ctx).Save(res).Error
}

func (r *aiAgentRepository) CreateApproval(ctx context.Context, approval *models.AIApproval) error {
	return r.conn(ctx).Create(approval).Error
}

func (r *aiAgentRepository) FindPendingApproval(ctx context.Context, planID, userID uuid.UUID) (*models.AIApproval, error) {
	var approval models.AIApproval
	if err := r.conn(ctx).First(&approval, "plan_id = ? AND user_id = ? AND status = ?", planID, userID, "pending").Error; err != nil {
		return nil, mapErr(err)
	}
	return &approval, nil
}

func (r *aiAgentRepository) SaveApproval(ctx context.Context, approval *models.AIApproval) error {
	return r.conn(ctx).Save(approval).Error
}

func (r *aiAgentRepository) CancelApprovals(ctx context.Context, planID, userID uuid.UUID, at *time.Time) error {
	// Consumed, superseded and expired approvals keep their history.
	return r.conn(ctx).Model(&models.AIApproval{}).
		Where("plan_id = ? AND user_id = ? AND status IN ?", planID, userID, []string{models.ApprovalApproved, models.ApprovalPending}).
		Updates(map[string]interface{}{
			"status":       "cancelled",
			"cancelled_at": at,
		}).Error
}
