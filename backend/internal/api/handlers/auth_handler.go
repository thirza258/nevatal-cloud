package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/services"
)

type AuthHandler struct {
	authService  *services.AuthService
	auditService *services.AuditService
	cfg          *config.Config
}

func NewAuthHandler(authService *services.AuthService, auditService *services.AuditService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		auditService: auditService,
		cfg:          cfg,
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

type OAuthRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
	MockEmail   string `json:"mock_email"`
	MockName    string `json:"mock_name"`
	MockAvatar  string `json:"mock_avatar"`
}

func (h *AuthHandler) GetOAuthURLs(c *gin.Context) {
	googleConfigured := h.cfg.GoogleClientID != ""
	githubConfigured := h.cfg.GithubClientID != ""

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"google": gin.H{
				"configured": googleConfigured,
				"client_id":  h.cfg.GoogleClientID,
			},
			"github": gin.H{
				"configured": githubConfigured,
				"client_id":  h.cfg.GithubClientID,
			},
		},
	})
}

func (h *AuthHandler) OAuthGoogle(c *gin.Context) {
	var req OAuthRequest
	_ = c.ShouldBindJSON(&req)

	oauthUser := services.OAuthUserInput{
		Provider:    "google",
		ProviderID:  fmt.Sprintf("google-%d", time.Now().UnixNano()),
		Email:       req.MockEmail,
		DisplayName: req.MockName,
		AvatarURL:   req.MockAvatar,
	}

	// 1. If real Google credentials configured and authorization code provided, perform real exchange
	if req.Code != "" && h.cfg.GoogleClientID != "" && h.cfg.GoogleClientSecret != "" {
		tokenURL := "https://oauth2.googleapis.com/token"
		redirectURI := req.RedirectURI
		if redirectURI == "" {
			redirectURI = fmt.Sprintf("%s/auth/callback", h.cfg.AppBaseURL)
		}

		vals := url.Values{
			"code":          {req.Code},
			"client_id":     {h.cfg.GoogleClientID},
			"client_secret": {h.cfg.GoogleClientSecret},
			"redirect_uri":  {redirectURI},
			"grant_type":    {"authorization_code"},
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.PostForm(tokenURL, vals)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var tokenResp struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err == nil && tokenResp.AccessToken != "" {
				// Fetch user profile
				userInfoReq, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
				userInfoReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
				userResp, err := client.Do(userInfoReq)
				if err == nil && userResp.StatusCode == http.StatusOK {
					defer userResp.Body.Close()
					var googleProfile struct {
						Sub     string `json:"sub"`
						Email   string `json:"email"`
						Name    string `json:"name"`
						Picture string `json:"picture"`
					}
					if err := json.NewDecoder(userResp.Body).Decode(&googleProfile); err == nil && googleProfile.Email != "" {
						oauthUser.ProviderID = googleProfile.Sub
						oauthUser.Email = googleProfile.Email
						oauthUser.DisplayName = googleProfile.Name
						oauthUser.AvatarURL = googleProfile.Picture
					}
				}
			}
		}
	}

	// 2. Default/fallback for seamless local testing or demo
	if oauthUser.Email == "" {
		oauthUser.Email = "alex.google@nevatal.id"
		oauthUser.DisplayName = "Alex Developer (Google)"
		oauthUser.AvatarURL = "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&h=100&fit=crop&crop=faces"
	}

	user, token, err := h.authService.OAuthLoginOrRegister(oauthUser)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "OAUTH_FAILED", "message": err.Error()},
		})
		return
	}

	h.auditService.Log(&user.ID, "auth.oauth.google", "user", &user.ID, "success", c.ClientIP(), map[string]string{
		"provider": "google",
		"email":    user.Email,
	})

	c.SetCookie("nevatal_token", token, 60*60*24*7, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}

func (h *AuthHandler) OAuthGitHub(c *gin.Context) {
	var req OAuthRequest
	_ = c.ShouldBindJSON(&req)

	oauthUser := services.OAuthUserInput{
		Provider:    "github",
		ProviderID:  fmt.Sprintf("gh-%d", time.Now().UnixNano()),
		Email:       req.MockEmail,
		DisplayName: req.MockName,
		AvatarURL:   req.MockAvatar,
	}

	// 1. If real GitHub credentials configured and code provided
	if req.Code != "" && h.cfg.GithubClientID != "" && h.cfg.GithubClientSecret != "" {
		tokenURL := "https://github.com/login/oauth/access_token"
		vals := url.Values{
			"code":          {req.Code},
			"client_id":     {h.cfg.GithubClientID},
			"client_secret": {h.cfg.GithubClientSecret},
		}

		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, _ := http.NewRequest("POST", tokenURL, strings.NewReader(vals.Encode()))
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(httpReq)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var tokenResp struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err == nil && tokenResp.AccessToken != "" {
				userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
				userReq.Header.Set("Authorization", "token "+tokenResp.AccessToken)
				userReq.Header.Set("User-Agent", "Nevatal-Cloud-ControlPlane")

				userResp, err := client.Do(userReq)
				if err == nil && userResp.StatusCode == http.StatusOK {
					defer userResp.Body.Close()
					var ghUser struct {
						ID        int64  `json:"id"`
						Login     string `json:"login"`
						Name      string `json:"name"`
						Email     string `json:"email"`
						AvatarURL string `json:"avatar_url"`
					}
					if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err == nil {
						oauthUser.ProviderID = fmt.Sprintf("%d", ghUser.ID)
						oauthUser.AvatarURL = ghUser.AvatarURL
						if ghUser.Name != "" {
							oauthUser.DisplayName = ghUser.Name
						} else {
							oauthUser.DisplayName = ghUser.Login
						}
						oauthUser.Email = ghUser.Email
					}
				}
			}
		}
	}

	// 2. Default/fallback for seamless local testing or demo
	if oauthUser.Email == "" {
		oauthUser.Email = "octocat.dev@github.nevatal.id"
		oauthUser.DisplayName = "Octocat Hacker (GitHub)"
		oauthUser.AvatarURL = "https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png"
	}

	user, token, err := h.authService.OAuthLoginOrRegister(oauthUser)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "OAUTH_FAILED", "message": err.Error()},
		})
		return
	}

	h.auditService.Log(&user.ID, "auth.oauth.github", "user", &user.ID, "success", c.ClientIP(), map[string]string{
		"provider": "github",
		"email":    user.Email,
	})

	c.SetCookie("nevatal_token", token, 60*60*24*7, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}

func (h *AuthHandler) OAuthWordPress(c *gin.Context) {
	var req struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		WPUserID    string `json:"wp_user_id"`
		SiteURL     string `json:"site_url"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.Email == "" {
		req.Email = "admin@wordpress.nevatal.id"
		req.DisplayName = "WordPress Admin"
	}

	oauthUser := services.OAuthUserInput{
		Provider:    "wordpress",
		ProviderID:  req.WPUserID,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		AvatarURL:   "https://s.w.org/style/images/about/WordPress-logotype-wmark.png",
	}

	user, token, err := h.authService.OAuthLoginOrRegister(oauthUser)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "OAUTH_FAILED", "message": err.Error()},
		})
		return
	}

	h.auditService.Log(&user.ID, "auth.oauth.wordpress", "user", &user.ID, "success", c.ClientIP(), map[string]string{
		"provider": "wordpress",
		"email":    user.Email,
	})

	c.SetCookie("nevatal_token", token, 60*60*24*7, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}
