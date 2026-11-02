// Package request holds the request DTOs (with their binding tags) and small
// helpers to read path and query parameters. One file per domain.
//
// Naming rule: a DTO keeps the exact Go type name the API bound into before
// the layered refactor, because gin/validator error messages embed it (for
// example "Key: 'CreateInstanceInput.Name' Error:..."). Bodies that used to be
// anonymous structs are declared as aliases of unnamed struct types
// (type X = struct{...}) so their messages stay unprefixed as well.
package request

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UUIDParam parses the path parameter name as a UUID.
func UUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Param(name))
}

// BindJSON decodes and validates the JSON body into dst.
func BindJSON(c *gin.Context, dst any) error {
	return c.ShouldBindJSON(dst)
}
