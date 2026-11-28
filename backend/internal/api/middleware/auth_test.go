package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireScopeEnforcesAPIKeyScopesAndAllowsJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	runReq := func(method string, scopes *string, resource string) int {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if scopes != nil {
				c.Set("apiKeyScopes", *scopes)
			}
			c.Next()
		})
		r.Use(RequireScope(resource))
		r.Any("/items", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/items", nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	readOnly := `["instances:read"]`
	if code := runReq(http.MethodGet, &readOnly, "instances"); code != http.StatusOK {
		t.Fatalf("expected GET with instances:read to succeed (200), got %d", code)
	}
	if code := runReq(http.MethodPost, &readOnly, "instances"); code != http.StatusForbidden {
		t.Fatalf("expected POST with instances:read to return 403, got %d", code)
	}
	if code := runReq(http.MethodDelete, &readOnly, "instances"); code != http.StatusForbidden {
		t.Fatalf("expected DELETE with instances:read to return 403, got %d", code)
	}
	if code := runReq(http.MethodGet, &readOnly, "networks"); code != http.StatusForbidden {
		t.Fatalf("expected GET networks with instances:read to return 403, got %d", code)
	}

	writeScope := `["instances:write"]`
	if code := runReq(http.MethodGet, &writeScope, "instances"); code != http.StatusOK {
		t.Fatalf("expected GET with instances:write to succeed (200), got %d", code)
	}
	if code := runReq(http.MethodPost, &writeScope, "instances"); code != http.StatusOK {
		t.Fatalf("expected POST with instances:write to succeed (200), got %d", code)
	}

	wildcardResource := `["networks:*"]`
	if code := runReq(http.MethodDelete, &wildcardResource, "networks"); code != http.StatusOK {
		t.Fatalf("expected DELETE networks with networks:* to succeed (200), got %d", code)
	}
	if code := runReq(http.MethodGet, &wildcardResource, "instances"); code != http.StatusForbidden {
		t.Fatalf("expected GET instances with networks:* to return 403, got %d", code)
	}

	globalWildcard := `["*"]`
	if code := runReq(http.MethodPost, &globalWildcard, "webhooks"); code != http.StatusOK {
		t.Fatalf("expected POST webhooks with * to succeed (200), got %d", code)
	}

	// JWT callers do not set apiKeyScopes and bypass API key scope checks.
	if code := runReq(http.MethodPost, nil, "webhooks"); code != http.StatusOK {
		t.Fatalf("expected JWT caller without apiKeyScopes to succeed (200), got %d", code)
	}
}
