package response

import (
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

// AuthSession is returned by register, login and the OAuth endpoints.
type AuthSession struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// OAuthURLs is the body of GET /auth/oauth/urls.
type OAuthURLs struct {
	DemoMode bool          `json:"demo_mode"`
	GitHub   OAuthProvider `json:"github"`
	Google   OAuthProvider `json:"google"`
}

type OAuthProvider struct {
	ClientID   string `json:"client_id"`
	Configured bool   `json:"configured"`
}

func NewOAuthURLs(p usecase.OAuthProviders) OAuthURLs {
	return OAuthURLs{
		DemoMode: p.DemoMode,
		GitHub:   OAuthProvider{ClientID: p.GitHubClientID, Configured: p.GitHubConfigured},
		Google:   OAuthProvider{ClientID: p.GoogleClientID, Configured: p.GoogleConfigured},
	}
}
