package usecase

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nevatal/cloud/internal/models"
)

// OAuthCodeInput is a social sign-in request from the SPA. The Mock* identity
// fields are honored only in DEMO_MODE; elsewhere identities come exclusively
// from the provider.
type OAuthCodeInput struct {
	Code        string
	RedirectURI string
	MockEmail   string
	MockName    string
	MockAvatar  string
}

// WordPressSSOInput is a sign-in asserted by the WordPress bridge plugin.
// APIKey is the shared secret the plugin presented.
type WordPressSSOInput struct {
	Email       string
	DisplayName string
	WPUserID    string
	SiteURL     string
	APIKey      string
}

// OAuthProviders reports which social providers are configured.
type OAuthProviders struct {
	GoogleConfigured bool
	GoogleClientID   string
	GitHubConfigured bool
	GitHubClientID   string
	DemoMode         bool
}

func (s *AuthUsecase) googleConfigured() bool {
	return s.cfg.GoogleClientID != "" && s.cfg.GoogleClientSecret != ""
}

func (s *AuthUsecase) githubConfigured() bool {
	return s.cfg.GithubClientID != "" && s.cfg.GithubClientSecret != ""
}

// OAuthProviders returns the public OAuth client settings.
func (s *AuthUsecase) OAuthProviders() OAuthProviders {
	return OAuthProviders{
		GoogleConfigured: s.googleConfigured(),
		GoogleClientID:   s.cfg.GoogleClientID,
		GitHubConfigured: s.githubConfigured(),
		GitHubClientID:   s.cfg.GithubClientID,
		DemoMode:         s.cfg.DemoMode,
	}
}

// checkOAuthPreconditions enforces the production rules for social login.
func (s *AuthUsecase) checkOAuthPreconditions(provider string, configured bool, req *OAuthCodeInput) error {
	if s.cfg.DemoMode {
		return nil
	}
	if !configured {
		return newError(KindUnavailable, "OAUTH_NOT_CONFIGURED", provider+" sign-in is not configured on this server")
	}
	if req.Code == "" {
		return newError(KindInvalid, "OAUTH_CODE_REQUIRED", "An OAuth authorization code is required")
	}
	if req.RedirectURI != "" && !strings.HasPrefix(req.RedirectURI, s.cfg.AppBaseURL+"/") {
		return newError(KindInvalid, "OAUTH_INVALID_REDIRECT", "redirect_uri must belong to APP_BASE_URL")
	}
	// Never trust identity fields supplied by the browser outside demo mode.
	req.MockEmail, req.MockName, req.MockAvatar = "", "", ""
	return nil
}

// completeOAuth signs the identity in and audits it as auth.oauth.<provider>.
func (s *AuthUsecase) completeOAuth(ctx context.Context, oauthUser OAuthUserInput, clientIP string) (*models.User, string, error) {
	user, token, err := s.OAuthLoginOrRegister(ctx, oauthUser)
	if err != nil {
		return nil, "", err
	}

	s.auditService.Log(ctx, &user.ID, "auth.oauth."+oauthUser.Provider, "user", &user.ID, "success", clientIP, map[string]string{
		"provider": oauthUser.Provider,
		"email":    user.Email,
	})
	return user, token, nil
}

// SignInWithGoogle exchanges a Google authorization code for a verified
// identity and signs it in. In demo mode a sample account is used when no
// verified identity is available.
func (s *AuthUsecase) SignInWithGoogle(ctx context.Context, req OAuthCodeInput, clientIP string) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	if err := s.checkOAuthPreconditions("Google", s.googleConfigured(), &req); err != nil {
		return nil, "", err
	}

	oauthUser := OAuthUserInput{
		Provider:    "google",
		ProviderID:  fmt.Sprintf("google-%d", time.Now().UnixNano()),
		Email:       req.MockEmail,
		DisplayName: req.MockName,
		AvatarURL:   req.MockAvatar,
	}

	// 1. If real Google credentials configured and authorization code provided, perform real exchange
	if req.Code != "" && s.googleConfigured() {
		tokenURL := "https://oauth2.googleapis.com/token"
		redirectURI := req.RedirectURI
		if redirectURI == "" {
			redirectURI = fmt.Sprintf("%s/auth/callback", s.cfg.AppBaseURL)
		}

		vals := url.Values{
			"code":          {req.Code},
			"client_id":     {s.cfg.GoogleClientID},
			"client_secret": {s.cfg.GoogleClientSecret},
			"redirect_uri":  {redirectURI},
			"grant_type":    {"authorization_code"},
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.PostForm(tokenURL, vals)
		if err == nil {
			defer resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			var tokenResp struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err == nil && tokenResp.AccessToken != "" {
				// Fetch user profile
				userInfoReq, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
				userInfoReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
				userResp, err := client.Do(userInfoReq)
				if err == nil {
					defer userResp.Body.Close()
				}
				if err == nil && userResp.StatusCode == http.StatusOK {
					var googleProfile struct {
						Sub           string `json:"sub"`
						Email         string `json:"email"`
						EmailVerified bool   `json:"email_verified"`
						Name          string `json:"name"`
						Picture       string `json:"picture"`
					}
					if err := json.NewDecoder(userResp.Body).Decode(&googleProfile); err == nil && googleProfile.Email != "" &&
						(googleProfile.EmailVerified || s.cfg.DemoMode) {
						oauthUser.ProviderID = googleProfile.Sub
						oauthUser.Email = googleProfile.Email
						oauthUser.DisplayName = googleProfile.Name
						oauthUser.AvatarURL = googleProfile.Picture
					}
				}
			}
		}
	}

	// 2. Without a verified identity: demo mode falls back to a sample account,
	// everything else is rejected.
	if oauthUser.Email == "" {
		if !s.cfg.DemoMode {
			return nil, "", newError(KindUnauthorized, "OAUTH_FAILED", "Google sign-in failed: the account could not be verified")
		}
		oauthUser.Email = "alex.google@nevatal.id"
		oauthUser.DisplayName = "Alex Developer (Google)"
		oauthUser.AvatarURL = "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&h=100&fit=crop&crop=faces"
	}

	return s.completeOAuth(ctx, oauthUser, clientIP)
}

// SignInWithGitHub exchanges a GitHub authorization code for the account's
// primary verified email and signs it in. In demo mode a sample account is
// used when no verified identity is available.
func (s *AuthUsecase) SignInWithGitHub(ctx context.Context, req OAuthCodeInput, clientIP string) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	if err := s.checkOAuthPreconditions("GitHub", s.githubConfigured(), &req); err != nil {
		return nil, "", err
	}

	oauthUser := OAuthUserInput{
		Provider:    "github",
		ProviderID:  fmt.Sprintf("gh-%d", time.Now().UnixNano()),
		Email:       req.MockEmail,
		DisplayName: req.MockName,
		AvatarURL:   req.MockAvatar,
	}

	// 1. If real GitHub credentials configured and code provided
	if req.Code != "" && s.githubConfigured() {
		tokenURL := "https://github.com/login/oauth/access_token"
		vals := url.Values{
			"code":          {req.Code},
			"client_id":     {s.cfg.GithubClientID},
			"client_secret": {s.cfg.GithubClientSecret},
		}

		client := &http.Client{Timeout: 10 * time.Second}
		httpReq, _ := http.NewRequest("POST", tokenURL, strings.NewReader(vals.Encode()))
		httpReq.Header.Set("Accept", "application/json")
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(httpReq)
		if err == nil {
			defer resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			var tokenResp struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err == nil && tokenResp.AccessToken != "" {
				userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
				userReq.Header.Set("Authorization", "token "+tokenResp.AccessToken)
				userReq.Header.Set("User-Agent", "Nevatal-Cloud-ControlPlane")

				userResp, err := client.Do(userReq)
				if err == nil {
					defer userResp.Body.Close()
				}
				if err == nil && userResp.StatusCode == http.StatusOK {
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
						// The profile email is user-editable and unverified; outside
						// demo mode use the primary verified address instead.
						oauthUser.Email = ""
						if s.cfg.DemoMode {
							oauthUser.Email = ghUser.Email
						}
						if verified := githubPrimaryVerifiedEmail(client, tokenResp.AccessToken); verified != "" {
							oauthUser.Email = verified
						}
					}
				}
			}
		}
	}

	// 2. Without a verified identity: demo mode falls back to a sample account,
	// everything else is rejected.
	if oauthUser.Email == "" {
		if !s.cfg.DemoMode {
			return nil, "", newError(KindUnauthorized, "OAUTH_FAILED", "GitHub sign-in failed: no verified primary email on the account")
		}
		oauthUser.Email = "octocat.dev@github.nevatal.id"
		oauthUser.DisplayName = "Octocat Hacker (GitHub)"
		oauthUser.AvatarURL = "https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png"
	}

	return s.completeOAuth(ctx, oauthUser, clientIP)
}

// githubPrimaryVerifiedEmail returns the primary verified address (needs the
// user:email scope) or "".
func githubPrimaryVerifiedEmail(client *http.Client, accessToken string) string {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Nevatal-Cloud-ControlPlane")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if json.NewDecoder(resp.Body).Decode(&emails) != nil {
		return ""
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email
		}
	}
	return ""
}

// SignInWithWordPress signs in a user asserted by the WordPress bridge plugin.
// Outside demo mode the request must carry the shared WORDPRESS_API_KEY, must
// name an email, and cannot sign in administrators.
func (s *AuthUsecase) SignInWithWordPress(ctx context.Context, req WordPressSSOInput, clientIP string) (*models.User, string, error) {
	ctx = context.WithoutCancel(ctx)

	if !s.cfg.DemoMode {
		if !s.cfg.WordPressEnabled() {
			return nil, "", newError(KindUnavailable, "WORDPRESS_DISABLED", "WordPress sign-in is not configured on this server")
		}
		if subtle.ConstantTimeCompare([]byte(req.APIKey), []byte(s.cfg.WordPressAPIKey)) != 1 {
			return nil, "", newError(KindUnauthorized, "UNAUTHORIZED", "Invalid or missing WordPress API key")
		}
		if strings.TrimSpace(req.Email) == "" {
			return nil, "", newError(KindInvalid, "BAD_REQUEST", "email is required")
		}
	}

	if req.Email == "" {
		req.Email = "admin@wordpress.nevatal.id"
		req.DisplayName = "WordPress Admin"
	}

	return s.completeOAuth(ctx, OAuthUserInput{
		Provider:    "wordpress",
		ProviderID:  req.WPUserID,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		AvatarURL:   "https://s.w.org/style/images/about/WordPress-logotype-wmark.png",
		DenyAdmin:   !s.cfg.DemoMode,
	}, clientIP)
}
