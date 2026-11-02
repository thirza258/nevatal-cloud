package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// writeError writes a use case error. A *usecase.Error keeps its code and
// gets the status of its kind; any other error is written with the
// endpoint's fallback status and code and its own message.
func writeError(c *gin.Context, err error, fallbackStatus int, fallbackCode string) {
	var ucErr *usecase.Error
	if errors.As(err, &ucErr) {
		if ucErr.Details != nil {
			response.ErrorWithDetails(c, statusForKind(ucErr.Kind, fallbackStatus), ucErr.Code, ucErr.Message, ucErr.Details)
			return
		}
		if ucErr.Field != "" {
			response.ErrorWithDetails(c, statusForKind(ucErr.Kind, fallbackStatus), ucErr.Code, ucErr.Message, response.FieldDetails{Field: ucErr.Field})
			return
		}
		response.Error(c, statusForKind(ucErr.Kind, fallbackStatus), ucErr.Code, ucErr.Message)
		return
	}
	response.Error(c, fallbackStatus, fallbackCode, err.Error())
}

func statusForKind(kind usecase.ErrorKind, fallback int) int {
	switch kind {
	case usecase.KindInvalid:
		return http.StatusBadRequest
	case usecase.KindUnauthorized:
		return http.StatusUnauthorized
	case usecase.KindForbidden:
		return http.StatusForbidden
	case usecase.KindNotFound:
		return http.StatusNotFound
	case usecase.KindConflict:
		return http.StatusConflict
	case usecase.KindUnprocessable:
		return http.StatusUnprocessableEntity
	case usecase.KindUnavailable:
		return http.StatusServiceUnavailable
	case usecase.KindBadGateway:
		return http.StatusBadGateway
	case usecase.KindTooManyRequests:
		return http.StatusTooManyRequests
	}
	return fallback
}

// writeMessageError is writeError for endpoints whose error body has no code
// ({"error": {"message": ...}}).
func writeMessageError(c *gin.Context, err error, fallbackStatus int) {
	var ucErr *usecase.Error
	if errors.As(err, &ucErr) {
		response.ErrorMessage(c, statusForKind(ucErr.Kind, fallbackStatus), ucErr.Message)
		return
	}
	response.ErrorMessage(c, fallbackStatus, err.Error())
}

// writeIfDenied handles endpoints that ignore operation errors but must still
// reject callers who do not own the resource: it writes a *usecase.Error
// (code-less body) and reports whether it did.
func writeIfDenied(c *gin.Context, err error) bool {
	var ucErr *usecase.Error
	if errors.As(err, &ucErr) {
		response.ErrorMessage(c, statusForKind(ucErr.Kind, http.StatusNotFound), ucErr.Message)
		return true
	}
	return false
}
