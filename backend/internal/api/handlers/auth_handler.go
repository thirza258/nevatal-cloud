package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AuthHandler struct {
	authService  *services.AuthService
	auditService *services.AuditService
}

func NewAuthHandler(authService *services.AuthService, auditService *services.AuditService) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		auditService: auditService,
	}
}

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	user, token, err := h.authService.Register(req.Email, req.Password, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": gin.H{"code": "REGISTRATION_FAILED", "message": err.Error()},
		})
		return
	}

	h.auditService.Log(&user.ID, "auth.register", "user", &user.ID, "success", c.ClientIP(), nil)

	// Set HttpOnly session cookie
	c.SetCookie("nevatal_token", token, 60*60*24*7, "/", "", false, true)

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	user, token, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		h.auditService.Log(nil, "auth.login_failed", "user", nil, "failure", c.ClientIP(), map[string]string{"email": req.Email})
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "INVALID_CREDENTIALS", "message": err.Error()},
		})
		return
	}

	h.auditService.Log(&user.ID, "auth.login", "user", &user.ID, "success", c.ClientIP(), nil)

	c.SetCookie("nevatal_token", token, 60*60*24*7, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.SetCookie("nevatal_token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"message": "Logged out successfully"},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{"code": "USER_NOT_FOUND", "message": "User not found"},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": user,
	})
}
