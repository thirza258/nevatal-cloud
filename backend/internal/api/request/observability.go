package request

import (
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

type CreateAlertRuleRequest struct {
	Name            string     `json:"name" binding:"required"`
	Description     string     `json:"description"`
	ResourceType    string     `json:"resource_type" binding:"required"`
	ResourceID      *uuid.UUID `json:"resource_id"`
	ConditionType   string     `json:"condition_type"`
	MetricName      string     `json:"metric_name"`
	Operator        string     `json:"operator"`
	Threshold       float64    `json:"threshold"`
	DurationSeconds int        `json:"duration_seconds"`
	Severity        string     `json:"severity"`
	Channels        []string   `json:"channels"`
	WebhookURL      string     `json:"webhook_url"`
	Enabled         *bool      `json:"enabled"`
}

func (r CreateAlertRuleRequest) ToInput() usecase.CreateAlertRuleInput {
	return usecase.CreateAlertRuleInput{
		Name:            r.Name,
		Description:     r.Description,
		ResourceType:    r.ResourceType,
		ResourceID:      r.ResourceID,
		ConditionType:   r.ConditionType,
		MetricName:      r.MetricName,
		Operator:        r.Operator,
		Threshold:       r.Threshold,
		DurationSeconds: r.DurationSeconds,
		Severity:        r.Severity,
		Channels:        r.Channels,
		WebhookURL:      r.WebhookURL,
		Enabled:         r.Enabled,
	}
}

type UpdateAlertRuleRequest struct {
	Name            *string   `json:"name"`
	Description     *string   `json:"description"`
	Operator        *string   `json:"operator"`
	Threshold       *float64  `json:"threshold"`
	DurationSeconds *int      `json:"duration_seconds"`
	Severity        *string   `json:"severity"`
	Channels        *[]string `json:"channels"`
	WebhookURL      *string   `json:"webhook_url"`
	Enabled         *bool     `json:"enabled"`
}

func (r UpdateAlertRuleRequest) ToInput() usecase.UpdateAlertRuleInput {
	return usecase.UpdateAlertRuleInput{
		Name:            r.Name,
		Description:     r.Description,
		Operator:        r.Operator,
		Threshold:       r.Threshold,
		DurationSeconds: r.DurationSeconds,
		Severity:        r.Severity,
		Channels:        r.Channels,
		WebhookURL:      r.WebhookURL,
		Enabled:         r.Enabled,
	}
}

type AcknowledgeAlertRequest struct {
	Note string `json:"note"`
}

type ResolveAlertRequest struct {
	Resolution string `json:"resolution"`
}

type CreateIncidentRequest struct {
	Title               string      `json:"title" binding:"required"`
	Severity            string      `json:"severity"`
	RootCause           string      `json:"root_cause"`
	Symptoms            []string    `json:"symptoms"`
	AffectedResourceIDs []uuid.UUID `json:"affected_resource_ids"`
	LeadActor           string      `json:"lead_actor"`
}

func (r CreateIncidentRequest) ToInput() usecase.CreateIncidentInput {
	return usecase.CreateIncidentInput{
		Title:               r.Title,
		Severity:            r.Severity,
		RootCause:           r.RootCause,
		Symptoms:            r.Symptoms,
		AffectedResourceIDs: r.AffectedResourceIDs,
		LeadActor:           r.LeadActor,
	}
}

type UpdateIncidentRequest struct {
	Status       *string `json:"status"`
	Severity     *string `json:"severity"`
	Title        *string `json:"title"`
	RootCause    *string `json:"root_cause"`
	Postmortem   *string `json:"postmortem"`
	TimelineNote *string `json:"timeline_note"`
}

func (r UpdateIncidentRequest) ToInput() usecase.UpdateIncidentInput {
	return usecase.UpdateIncidentInput{
		Status:       r.Status,
		Severity:     r.Severity,
		Title:        r.Title,
		RootCause:    r.RootCause,
		Postmortem:   r.Postmortem,
		TimelineNote: r.TimelineNote,
	}
}

type AIInvestigateRequest struct {
	IncidentID      *uuid.UUID `json:"incident_id"`
	ResourceID      *uuid.UUID `json:"resource_id"`
	Query           string     `json:"query"`
	LookbackMinutes int        `json:"lookback_minutes"`
}

func (r AIInvestigateRequest) ToInput() usecase.AIInvestigateInput {
	return usecase.AIInvestigateInput{
		IncidentID:      r.IncidentID,
		ResourceID:      r.ResourceID,
		Query:           r.Query,
		LookbackMinutes: r.LookbackMinutes,
	}
}
