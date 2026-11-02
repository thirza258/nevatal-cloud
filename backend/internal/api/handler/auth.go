package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

const sessionCookieMaxAge = 60 * 60 * 24 * 7

type AuthHandler struct {
	auth *usecase.AuthUsecase
	cfg  *config.Config
}

func NewAuthHandler(auth *usecase.AuthUsecase, cfg *config.Config) *AuthHandler {
	return &AuthHandler{auth: auth, cfg: cfg}
}

// setSessionCookie writes the HttpOnly session cookie. Secure is on whenever
// APP_BASE_URL is https (or COOKIE_SECURE=true); then the cookie is named
// __Host-nevatal_token so sibling subdomains cannot toss it.
func (h *AuthHandler) setSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(middleware.SessionCookieName(h.cfg.CookieSecure), token, maxAge, "/", "", h.cfg.CookieSecure, true)
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	user, token, err := h.auth.Register(c.Request.Context(), req.Email, req.Password, req.DisplayName, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusConflict, "REGISTRATION_FAILED", err.Error())
		return
	}

	// Set HttpOnly session cookie
	h.setSessionCookie(c, token, sessionCookieMaxAge)
	response.Created(c, response.AuthSession{Token: token, User: user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	user, token, err := h.auth.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", err.Error())
		return
	}

	h.setSessionCookie(c, token, sessionCookieMaxAge)
	response.OK(c, response.AuthSession{Token: token, User: user})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	h.setSessionCookie(c, "", -1)
	if h.cfg.CookieSecure {
		// Also expire a cookie left over from before the __Host- rename.
		c.SetCookie(middleware.LegacySessionCookie, "", -1, "/", "", true, true)
	}
	response.OKMessage(c, "Logged out successfully")
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.auth.GetUserByID(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}
	response.OK(c, user)
}

func (h *AuthHandler) GetOAuthURLs(c *gin.Context) {
	response.OK(c, response.NewOAuthURLs(h.auth.OAuthProviders()))
}

func (h *AuthHandler) OAuthGoogle(c *gin.Context) {
	var req request.OAuthRequest
	_ = c.ShouldBindJSON(&req)

	user, token, err := h.auth.SignInWithGoogle(c.Request.Context(), req.ToUsecase(), c.ClientIP())
	h.writeSession(c, user, token, err)
}

func (h *AuthHandler) OAuthGitHub(c *gin.Context) {
	var req request.OAuthRequest
	_ = c.ShouldBindJSON(&req)

	user, token, err := h.auth.SignInWithGitHub(c.Request.Context(), req.ToUsecase(), c.ClientIP())
	h.writeSession(c, user, token, err)
}

// OAuthWordPress signs in a user asserted by the WordPress bridge plugin.
// Outside demo mode the request must carry the shared WORDPRESS_API_KEY in
// X-Nevatal-Key, must name an email, and cannot sign in administrators.
func (h *AuthHandler) OAuthWordPress(c *gin.Context) {
	var req request.WordPressSSORequest
	_ = c.ShouldBindJSON(&req)

	user, token, err := h.auth.SignInWithWordPress(c.Request.Context(), request.WordPressSSOInput(req, c.GetHeader("X-Nevatal-Key")), c.ClientIP())
	h.writeSession(c, user, token, err)
}

// writeSession finishes a social sign-in: sets the session cookie and returns
// the session, or reports the error (422 OAUTH_FAILED unless the use case
// classified it).
func (h *AuthHandler) writeSession(c *gin.Context, user *models.User, token string, err error) {
	if err != nil {
		writeError(c, err, http.StatusUnprocessableEntity, "OAUTH_FAILED")
		return
	}

	h.setSessionCookie(c, token, sessionCookieMaxAge)
	response.OK(c, response.AuthSession{Token: token, User: user})
}
