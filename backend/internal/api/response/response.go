// Package response writes HTTP response bodies. Success bodies use the
// envelope {"data": ...}; errors use {"error": {"code", "message"[, "details"]}}.
//
// Composite payloads are typed DTOs (one file per domain). Their fields are
// declared in alphabetical order of their JSON keys so the encoded bytes match
// the map-based (gin.H) bodies the API returned before the layered refactor.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the success body.
type Envelope struct {
	Data any `json:"data"`
}

// ErrorBody is the object inside an error envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
	Message string `json:"message"`
}

// FieldDetails are the details of a validation error about one request
// field: {"field": "target_port"}.
type FieldDetails struct {
	Field string `json:"field"`
}

// ErrorEnvelope is the error body.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// MessageErrorEnvelope is the legacy error body without a code, used by the
// game server and Android endpoints: {"error": {"message": ...}}.
type MessageErrorEnvelope struct {
	Error Message `json:"error"`
}

// Message is a body that only carries a human-readable message. It is used
// both inside the envelope ({"data": {"message": ...}}) and, for a few legacy
// endpoints, as the whole body.
type Message struct {
	Message string `json:"message"`
}

// OK writes 200 {"data": data}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Data: data})
}

// Created writes 201 {"data": data}.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Data: data})
}

// Accepted writes 202 {"data": data}.
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, Envelope{Data: data})
}

// NoContent writes 204 without a body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// OKMessage writes 200 {"data": {"message": message}}.
func OKMessage(c *gin.Context, message string) {
	OK(c, Message{Message: message})
}

// Error writes {"error": {"code": code, "message": message}} with status.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// ErrorWithDetails writes an error envelope that also carries details.
func ErrorWithDetails(c *gin.Context, status int, code, message string, details any) {
	c.JSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message, Details: details}})
}

// ErrorMessage writes the legacy code-less error body
// {"error": {"message": message}} with status.
func ErrorMessage(c *gin.Context, status int, message string) {
	c.JSON(status, MessageErrorEnvelope{Error: Message{Message: message}})
}

// AbortError writes an error envelope and stops the handler chain
// (for middleware).
func AbortError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

// JSON writes body as is, without the envelope. Only for endpoints whose
// public contract predates the envelope (health check, WordPress bridge,
// a few admin ISO actions).
func JSON(c *gin.Context, status int, body any) {
	c.JSON(status, body)
}
