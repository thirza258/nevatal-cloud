package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"

	CtxKeyRequestID     = "request_id"
	CtxKeyCorrelationID = "correlation_id"
)

// CorrelationMiddleware extracts or issues X-Request-ID and X-Correlation-ID
// headers and binds them to the request context. It also records API metrics.
func CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(HeaderRequestID)
		if strings.TrimSpace(reqID) == "" {
			reqID = "req_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
		}

		corrID := c.GetHeader(HeaderCorrelationID)
		if strings.TrimSpace(corrID) == "" {
			corrID = "c_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
		}

		c.Set(CtxKeyRequestID, reqID)
		c.Set(CtxKeyCorrelationID, corrID)

		c.Writer.Header().Set(HeaderRequestID, reqID)
		c.Writer.Header().Set(HeaderCorrelationID, corrID)

		start := time.Now()

		c.Next()

		// Skip health probes from API metrics calculation
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		if path != "/health" {
			usecase.GetGlobalAPIMetrics().Record(usecase.APIMetricSample{
				Timestamp:  start,
				Method:     c.Request.Method,
				Path:       path,
				StatusCode: c.Writer.Status(),
				Duration:   time.Since(start),
			})
		}
	}
}

// GetRequestID returns the request ID from the Gin context or generates one.
func GetRequestID(c *gin.Context) string {
	if val, ok := c.Get(CtxKeyRequestID); ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return "req_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
}

// GetCorrelationID returns the correlation ID from the Gin context or generates one.
func GetCorrelationID(c *gin.Context) string {
	if val, ok := c.Get(CtxKeyCorrelationID); ok {
		if s, ok := val.(string); ok && s != "" {
			return s
		}
	}
	return "c_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
}

type contextKey string

const (
	ctxKeyReqID  contextKey = "request_id"
	ctxKeyCorrID contextKey = "correlation_id"
)

// WithCorrelation attaches correlation metadata to a standard Go context.
func WithCorrelation(ctx context.Context, reqID, corrID string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyReqID, reqID)
	return context.WithValue(ctx, ctxKeyCorrID, corrID)
}

// FromContext extracts the correlation ID from a context if present.
func FromContext(ctx context.Context) string {
	if val, ok := ctx.Value(ctxKeyCorrID).(string); ok {
		return val
	}
	return ""
}
