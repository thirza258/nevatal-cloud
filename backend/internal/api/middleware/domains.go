package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/config"
)

// RequireDomains answers 503 DOMAINS_DISABLED on the Nevatal Domains routes
// when the feature is off (PUBLIC_DOMAIN_ROOT unset, or DEMO_MODE=true).
func RequireDomains(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.DomainsEnabled() {
			response.AbortError(c, http.StatusServiceUnavailable, "DOMAINS_DISABLED", "Nevatal Domains is not enabled on this server")
			return
		}
		c.Next()
	}
}
