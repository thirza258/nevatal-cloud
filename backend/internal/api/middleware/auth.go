package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

const (
	// LegacySessionCookie is used on plain-HTTP (development) deployments.
	LegacySessionCookie = "nevatal_token"
	// SecureSessionCookie is used whenever cookies are Secure. The __Host-
	// prefix forces Secure, Path=/ and no Domain attribute, so sibling
	// subdomains (tenant apps on *.<PUBLIC_DOMAIN_ROOT>) cannot set or
	// overwrite it.
	SecureSessionCookie = "__Host-nevatal_token"
)

// SessionCookieName returns the session cookie name for the deployment.
func SessionCookieName(secure bool) string {
	if secure {
		return SecureSessionCookie
	}
	return LegacySessionCookie
}

// AuthRequired accepts a Bearer token, an X-API-Key header, or the session cookie
// on safe methods only, and stores the caller's id, email and role in the gin context.
func AuthRequired(authService *usecase.AuthUsecase, devService *usecase.DeveloperUsecase, secureCookies bool) gin.HandlerFunc {
	cookieName := SessionCookieName(secureCookies)
	return func(c *gin.Context) {
		var tokenStr string

		apiKeyHeader := c.GetHeader("X-API-Key")
		if apiKeyHeader != "" && strings.HasPrefix(apiKeyHeader, "nv_live_") {
			tokenStr = apiKeyHeader
		} else {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			} else if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
				cookieToken, err := c.Cookie(cookieName)
				if err == nil && cookieToken != "" {
					tokenStr = cookieToken
				}
			}
		}

		if tokenStr == "" {
			response.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required to access this resource")
			return
		}

		// Check if caller authenticated via customer API key
		if devService != nil && strings.HasPrefix(tokenStr, "nv_live_") {
			apiKey, err := devService.ValidateRawAPIKey(c.Request.Context(), tokenStr, "")
			if err != nil {
				response.AbortError(c, http.StatusUnauthorized, "INVALID_API_KEY", err.Error())
				return
			}
			c.Set("userID", apiKey.TenantID)
			c.Set("email", "api-key@"+apiKey.KeyPrefix)
			c.Set("role", models.RoleCustomer)
			c.Set("apiKeyScopes", apiKey.Scopes)
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(tokenStr)
		if err != nil {
			response.AbortError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Session expired or invalid token")
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists || roleVal != models.RoleAdmin {
			response.AbortError(c, http.StatusForbidden, "FORBIDDEN", "Administrator privileges required")
			return
		}
		c.Next()
	}
}

func GetCurrentUserID(c *gin.Context) uuid.UUID {
	val, ok := c.Get("userID")
	if !ok {
		return uuid.Nil
	}
	return val.(uuid.UUID)
}

func IsAdmin(c *gin.Context) bool {
	val, ok := c.Get("role")
	if !ok {
		return false
	}
	return val == models.RoleAdmin
}
