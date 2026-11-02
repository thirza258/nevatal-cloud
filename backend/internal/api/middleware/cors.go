package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/config"
)

const (
	corsAllowHeaders  = "Origin, Content-Length, Content-Type, Authorization, X-Idempotency-Key, X-Requested-With, X-Nevatal-Key"
	corsAllowMethods  = "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS"
	corsExposeHeaders = "Content-Length, Content-Type, Content-Disposition"
)

// SetupCORS reflects any origin. Kept for local development and tests only;
// the router uses SetupCORSWithConfig.
func SetupCORS() gin.HandlerFunc {
	return corsHandler(nil, true)
}

// SetupCORSWithConfig only grants cross-origin access (with credentials) to the
// origins in CORS_ALLOWED_ORIGINS. In production that list defaults to the
// APP_BASE_URL origin. Development without an explicit list reflects any
// origin so the Vite dev server and local tools keep working.
func SetupCORSWithConfig(cfg *config.Config) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.CORSAllowedOrigins))
	for _, origin := range cfg.CORSAllowedOrigins {
		allowed[normalizeOrigin(origin)] = struct{}{}
	}
	allowAny := len(allowed) == 0 && !cfg.IsProduction()
	return corsHandler(allowed, allowAny)
}

func corsHandler(allowed map[string]struct{}, allowAny bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Add("Vary", "Origin")
			if _, ok := allowed[normalizeOrigin(origin)]; ok || allowAny {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", corsAllowHeaders)
				c.Header("Access-Control-Allow-Methods", corsAllowMethods)
				c.Header("Access-Control-Expose-Headers", corsExposeHeaders)
				c.Header("Access-Control-Max-Age", "600")
			}
		}

		if c.Request.Method == http.MethodOptions {
			// Disallowed origins get a bare 204 without CORS headers, which the
			// browser treats as a failed preflight.
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func normalizeOrigin(origin string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(origin), "/"))
}
