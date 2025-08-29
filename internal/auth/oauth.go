package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/vfa-khuongdv/go-backup-drive/internal/database"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Service handles OAuth2 authentication for Google Drive API
type Service struct {
	config    *oauth2.Config
	dbService *database.Service
}

// TokenInfo represents token information for display
type TokenInfo struct {
	HasToken bool      `json:"has_token"`
	Expiry   time.Time `json:"expiry,omitempty"`
	Valid    bool      `json:"valid"`
}

// NewService creates a new auth service
func NewService(clientID, clientSecret string, RedirectURL string, dbService *database.Service) *Service {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  RedirectURL,
		Scopes:       []string{drive.DriveFileScope},
		Endpoint:     google.Endpoint,
	}

	return &Service{
		config:    config,
		dbService: dbService,
	}
}

// GetAuthURL returns the authorization URL for OAuth2 flow
func (s *Service) GetAuthURL() string {
	return s.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeToken exchanges authorization code for tokens and saves them
func (s *Service) ExchangeToken(authCode string) error {
	token, err := s.config.Exchange(context.Background(), authCode)
	if err != nil {
		return fmt.Errorf("failed to exchange token: %w", err)
	}

	// Save token to database
	tokenConfig := &database.TokenConfig{
		ClientID:     s.config.ClientID,
		ClientSecret: s.config.ClientSecret,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}

	if err := s.dbService.SaveTokenConfig(tokenConfig); err != nil {
		return fmt.Errorf("failed to save token config: %w", err)
	}

	return nil
}

// GetValidToken returns a valid token, refreshing if necessary
func (s *Service) GetValidToken() (*oauth2.Token, error) {
	// Get stored token
	tokenConfig, err := s.dbService.GetTokenConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get stored token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tokenConfig.AccessToken,
		RefreshToken: tokenConfig.RefreshToken,
		TokenType:    tokenConfig.TokenType,
		Expiry:       tokenConfig.Expiry,
	}

	// Check if token needs refresh
	if token.Expiry.Before(time.Now().Add(5 * time.Minute)) {
		tokenSource := s.config.TokenSource(context.Background(), token)
		newToken, err := tokenSource.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}

		// Update stored token if it was refreshed
		if newToken.AccessToken != token.AccessToken {
			tokenConfig.AccessToken = newToken.AccessToken
			tokenConfig.RefreshToken = newToken.RefreshToken
			tokenConfig.Expiry = newToken.Expiry

			if err := s.dbService.SaveTokenConfig(tokenConfig); err != nil {
				return nil, fmt.Errorf("failed to save refreshed token: %w", err)
			}
		}

		return newToken, nil
	}

	return token, nil
}

// GetClient returns an authenticated HTTP client
func (s *Service) GetClient() (*oauth2.Config, *oauth2.Token, error) {
	token, err := s.GetValidToken()
	if err != nil {
		return nil, nil, err
	}
	return s.config, token, nil
}

// GetTokenInfo returns information about the current token
func (s *Service) GetTokenInfo() (*TokenInfo, error) {
	tokenConfig, err := s.dbService.GetTokenConfig()
	if err != nil {
		return &TokenInfo{HasToken: false}, nil
	}

	info := &TokenInfo{
		HasToken: true,
		Expiry:   tokenConfig.Expiry,
		Valid:    time.Now().Before(tokenConfig.Expiry),
	}

	return info, nil
}

// ValidateToken validates the stored token by making a test API call
func (s *Service) ValidateToken() error {
	_, token, err := s.GetClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	client := s.config.Client(ctx, token)

	// Create Drive service and test with a simple API call
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create drive service: %w", err)
	}

	// Test API call - get user info
	_, err = driveService.About.Get().Fields("user").Do()
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	return nil
}
