package request

import "github.com/nevatal/cloud/internal/usecase"

// RegisterRequest is the body of POST /auth/register.
type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name" binding:"required"`
}

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// OAuthRequest carries an authorization code. The mock_* fields are honored
// only in DEMO_MODE; elsewhere identities come exclusively from the provider.
type OAuthRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
	MockEmail   string `json:"mock_email"`
	MockName    string `json:"mock_name"`
	MockAvatar  string `json:"mock_avatar"`
}

func (r OAuthRequest) ToUsecase() usecase.OAuthCodeInput {
	return usecase.OAuthCodeInput(r)
}

// WordPressSSORequest is the body of POST /auth/oauth/wordpress.
type WordPressSSORequest = struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	WPUserID    string `json:"wp_user_id"`
	SiteURL     string `json:"site_url"`
}

// WordPressSSOInput builds the use case input from the body and the API key
// the plugin presented in X-Nevatal-Key.
func WordPressSSOInput(r WordPressSSORequest, apiKey string) usecase.WordPressSSOInput {
	return usecase.WordPressSSOInput{
		Email:       r.Email,
		DisplayName: r.DisplayName,
		WPUserID:    r.WPUserID,
		SiteURL:     r.SiteURL,
		APIKey:      apiKey,
	}
}
