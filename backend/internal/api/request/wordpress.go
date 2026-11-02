package request

import "github.com/nevatal/cloud/internal/usecase"

// WPWebhookPayload is the body of POST /webhooks/wordpress.
type WPWebhookPayload struct {
	Action      string `json:"action" binding:"required"` // ping, provision_vps, list_instances, control_instance, get_catalog
	UserEmail   string `json:"user_email"`
	UserName    string `json:"user_name"`
	PlanID      string `json:"plan_id"`
	TemplateID  string `json:"template_id"`
	InstanceID  string `json:"instance_id"`
	PowerAction string `json:"power_action"` // start, shutdown, restart, delete
	CustomName  string `json:"custom_name"`
	OrderID     string `json:"order_id"` // WooCommerce Order ID
}

func (r WPWebhookPayload) ProvisionInput() usecase.WordPressProvisionInput {
	return usecase.WordPressProvisionInput{
		UserEmail:  r.UserEmail,
		UserName:   r.UserName,
		PlanID:     r.PlanID,
		TemplateID: r.TemplateID,
		CustomName: r.CustomName,
		OrderID:    r.OrderID,
	}
}
