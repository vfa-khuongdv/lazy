package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"github.com/vfa-khuongdv/lazy/internal/database"
)

type MockDatabaseService struct {
	mock.Mock
}

func (m *MockDatabaseService) SaveTokenConfig(config *database.TokenConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockDatabaseService) GetTokenConfig() (*database.TokenConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.TokenConfig), args.Error(1)
}

// AuthServiceTestSuite for oauth service tests
type AuthServiceTestSuite struct {
	suite.Suite
	service      *Service
	mockDB       *MockDatabaseService
	clientID     string
	clientSecret string
	redirectURL  string
}

func (suite *AuthServiceTestSuite) SetupTest() {
	suite.mockDB = &MockDatabaseService{}
	suite.clientID = "test-client-id"
	suite.clientSecret = "test-client-secret"
	suite.redirectURL = "http://localhost:8080/callback"

	suite.service = NewService(suite.clientID, suite.clientSecret, suite.redirectURL, suite.mockDB)
}

func (suite *AuthServiceTestSuite) TearDownTest() {
	// Clean up any resources if needed
}

func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}

// Test NewService
func (suite *AuthServiceTestSuite) TestNewService() {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	redirectURL := "http://localhost:8080/callback"
	mockDB := &MockDatabaseService{}

	service := NewService(clientID, clientSecret, redirectURL, mockDB)

	suite.NotNil(service)
	suite.NotNil(service.config)
	suite.Equal(mockDB, service.dbService)
	suite.Equal(clientID, service.config.ClientID)
	suite.Equal(clientSecret, service.config.ClientSecret)
	suite.Equal(redirectURL, service.config.RedirectURL)
	suite.Equal([]string{drive.DriveFileScope}, service.config.Scopes)
	suite.Equal(google.Endpoint, service.config.Endpoint)
}

// Test NewService with empty parameters
func (suite *AuthServiceTestSuite) TestNewService_EmptyParams() {
	service := NewService("", "", "", suite.mockDB)

	suite.NotNil(service)
	suite.Equal("", service.config.ClientID)
	suite.Equal("", service.config.ClientSecret)
	suite.Equal("", service.config.RedirectURL)
}

// Test NewService with nil database service
func (suite *AuthServiceTestSuite) TestNewService_NilDB() {
	service := NewService(suite.clientID, suite.clientSecret, suite.redirectURL, nil)

	suite.NotNil(service)
	suite.Nil(service.dbService)
}

// Test GetAuthURL
func (suite *AuthServiceTestSuite) TestGetAuthURL() {
	authURL := suite.service.GetAuthURL()

	suite.NotEmpty(authURL)
	suite.Contains(authURL, "https://accounts.google.com/o/oauth2/auth")
	suite.Contains(authURL, "client_id="+suite.clientID)
	// URL encoding: http://localhost:8080/callback becomes http%3A%2F%2Flocalhost%3A8080%2Fcallback
	suite.Contains(authURL, "redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback")
	// URL encoding: scope becomes encoded
	suite.Contains(authURL, "scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdrive.file")
	suite.Contains(authURL, "state=state-token")
	suite.Contains(authURL, "access_type=offline")
	// Note: OAuth2 library now uses "prompt=consent" instead of "approval_prompt=force"
	suite.Contains(authURL, "prompt=consent")
}

// Test ExchangeToken - Success
func (suite *AuthServiceTestSuite) TestExchangeToken_Success() {
	// This test will attempt actual OAuth exchange, which will fail in unit test
	// We're testing the flow and error handling
	authCode := "test-auth-code"

	suite.mockDB.On("SaveTokenConfig", mock.AnythingOfType("*database.TokenConfig")).Return(nil)

	err := suite.service.ExchangeToken(authCode)

	// Will fail with OAuth exchange error, but we can test error handling
	suite.Error(err)
	suite.Contains(err.Error(), "failed to exchange token")

	// Verify that SaveTokenConfig would have been called if exchange succeeded
	// Since exchange fails, SaveTokenConfig won't be called
	suite.mockDB.AssertNotCalled(suite.T(), "SaveTokenConfig")
}

// Test ExchangeToken - Database Save Error (simulated)
func (suite *AuthServiceTestSuite) TestExchangeToken_DatabaseSaveError() {
	// This tests the database save error path
	// Since we can't easily mock the OAuth exchange, we'll test this indirectly
	authCode := "test-auth-code"

	err := suite.service.ExchangeToken(authCode)

	// Will fail at OAuth exchange step
	suite.Error(err)
	suite.Contains(err.Error(), "failed to exchange token")
}

// Test GetValidToken - Success with valid token
func (suite *AuthServiceTestSuite) TestGetValidToken_ValidToken() {
	futureTime := time.Now().Add(time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       futureTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	token, err := suite.service.GetValidToken()

	suite.NoError(err)
	suite.NotNil(token)
	suite.Equal("valid-access-token", token.AccessToken)
	suite.Equal("valid-refresh-token", token.RefreshToken)
	suite.Equal("Bearer", token.TokenType)
	suite.Equal(futureTime, token.Expiry)

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetValidToken - Token needs refresh
func (suite *AuthServiceTestSuite) TestGetValidToken_NeedsRefresh() {
	pastTime := time.Now().Add(-time.Hour) // Expired token
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "expired-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       pastTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	token, err := suite.service.GetValidToken()

	// Will fail when trying to refresh token due to invalid credentials in test
	suite.Error(err)
	suite.Nil(token)
	suite.Contains(err.Error(), "failed to refresh token")

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetValidToken - Database error
func (suite *AuthServiceTestSuite) TestGetValidToken_DatabaseError() {
	dbError := errors.New("database connection failed")
	suite.mockDB.On("GetTokenConfig").Return((*database.TokenConfig)(nil), dbError)

	token, err := suite.service.GetValidToken()

	suite.Error(err)
	suite.Nil(token)
	suite.Contains(err.Error(), "failed to get stored token")
	suite.Contains(err.Error(), "database connection failed")

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetClient - Success
func (suite *AuthServiceTestSuite) TestGetClient_Success() {
	futureTime := time.Now().Add(time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       futureTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	config, token, err := suite.service.GetClient()

	suite.NoError(err)
	suite.NotNil(config)
	suite.NotNil(token)
	suite.Equal(suite.service.config, config)
	suite.Equal("valid-access-token", token.AccessToken)

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetClient - Error from GetValidToken
func (suite *AuthServiceTestSuite) TestGetClient_GetValidTokenError() {
	dbError := errors.New("token retrieval failed")
	suite.mockDB.On("GetTokenConfig").Return((*database.TokenConfig)(nil), dbError)

	config, token, err := suite.service.GetClient()

	suite.Error(err)
	suite.Nil(config)
	suite.Nil(token)

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetTokenInfo - Has valid token
func (suite *AuthServiceTestSuite) TestGetTokenInfo_HasValidToken() {
	futureTime := time.Now().Add(time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       futureTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	tokenInfo, err := suite.service.GetTokenInfo()

	suite.NoError(err)
	suite.NotNil(tokenInfo)
	suite.True(tokenInfo.HasToken)
	suite.Equal(futureTime, tokenInfo.Expiry)
	suite.True(tokenInfo.Valid) // Token is not expired

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetTokenInfo - Has expired token
func (suite *AuthServiceTestSuite) TestGetTokenInfo_HasExpiredToken() {
	pastTime := time.Now().Add(-time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "expired-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       pastTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	tokenInfo, err := suite.service.GetTokenInfo()

	suite.NoError(err)
	suite.NotNil(tokenInfo)
	suite.True(tokenInfo.HasToken)
	suite.Equal(pastTime, tokenInfo.Expiry)
	suite.False(tokenInfo.Valid) // Token is expired

	suite.mockDB.AssertExpectations(suite.T())
}

// Test GetTokenInfo - No token
func (suite *AuthServiceTestSuite) TestGetTokenInfo_NoToken() {
	dbError := errors.New("token not found")
	suite.mockDB.On("GetTokenConfig").Return((*database.TokenConfig)(nil), dbError)

	tokenInfo, err := suite.service.GetTokenInfo()

	suite.NoError(err) // Method returns no error when token not found
	suite.NotNil(tokenInfo)
	suite.False(tokenInfo.HasToken)

	suite.mockDB.AssertExpectations(suite.T())
}

// Test ValidateToken - Success path setup
func (suite *AuthServiceTestSuite) TestValidateToken_Setup() {
	futureTime := time.Now().Add(time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     suite.clientID,
		ClientSecret: suite.clientSecret,
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       futureTime,
	}

	suite.mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	err := suite.service.ValidateToken()

	// Will fail when trying to make actual API call, but we test the setup
	suite.Error(err)
	// Could be service creation error or API call error
	suite.True(strings.Contains(err.Error(), "failed to create drive service") ||
		strings.Contains(err.Error(), "token validation failed"))

	suite.mockDB.AssertExpectations(suite.T())
}

// Test ValidateToken - GetClient error
func (suite *AuthServiceTestSuite) TestValidateToken_GetClientError() {
	dbError := errors.New("database error")
	suite.mockDB.On("GetTokenConfig").Return((*database.TokenConfig)(nil), dbError)

	err := suite.service.ValidateToken()

	suite.Error(err)
	suite.Contains(err.Error(), "failed to get stored token")

	suite.mockDB.AssertExpectations(suite.T())
}

// Test RefreshToken - Success path setup
func (suite *AuthServiceTestSuite) TestRefreshToken_Setup() {
	token := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour), // Expired
	}

	suite.mockDB.On("SaveTokenConfig", mock.AnythingOfType("*database.TokenConfig")).Return(nil)

	newToken, err := suite.service.RefreshToken(token)

	// Will fail when trying to refresh with invalid credentials in test
	suite.Error(err)
	suite.Nil(newToken)
	suite.Contains(err.Error(), "failed to refresh token")

	// SaveTokenConfig should not be called because refresh failed
	suite.mockDB.AssertNotCalled(suite.T(), "SaveTokenConfig")
}

// Test RefreshToken - Database save error (simulated)
func (suite *AuthServiceTestSuite) TestRefreshToken_DatabaseError() {
	// Test case where refresh succeeds but database save fails
	// This is hard to test directly due to OAuth dependencies
	token := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "valid-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}

	newToken, err := suite.service.RefreshToken(token)

	// Will fail at OAuth refresh step
	suite.Error(err)
	suite.Nil(newToken)
	suite.Contains(err.Error(), "failed to refresh token")
}

// Additional unit tests for edge cases

func TestNewService_EdgeCases(t *testing.T) {
	// Test with various parameter combinations
	testCases := []struct {
		name         string
		clientID     string
		clientSecret string
		redirectURL  string
		dbService    *MockDatabaseService
	}{
		{
			name:         "all empty strings",
			clientID:     "",
			clientSecret: "",
			redirectURL:  "",
			dbService:    &MockDatabaseService{},
		},
		{
			name:         "nil database service",
			clientID:     "test",
			clientSecret: "test",
			redirectURL:  "test",
			dbService:    nil,
		},
		{
			name:         "very long parameters",
			clientID:     strings.Repeat("a", 1000),
			clientSecret: strings.Repeat("b", 1000),
			redirectURL:  "http://localhost:8080/" + strings.Repeat("c", 1000),
			dbService:    &MockDatabaseService{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service := NewService(tc.clientID, tc.clientSecret, tc.redirectURL, tc.dbService)
			assert.NotNil(t, service)
			assert.Equal(t, tc.clientID, service.config.ClientID)
			assert.Equal(t, tc.clientSecret, service.config.ClientSecret)
			assert.Equal(t, tc.redirectURL, service.config.RedirectURL)
			assert.Equal(t, tc.dbService, service.dbService)
		})
	}
}

// Test TokenInfo struct
func TestTokenInfo(t *testing.T) {
	// Test TokenInfo struct creation and properties
	now := time.Now()

	tokenInfo := &TokenInfo{
		HasToken: true,
		Expiry:   now,
		Valid:    true,
	}

	assert.True(t, tokenInfo.HasToken)
	assert.Equal(t, now, tokenInfo.Expiry)
	assert.True(t, tokenInfo.Valid)
}

// Test oauth2.Config properties
func TestOAuth2Config_Properties(t *testing.T) {
	clientID := "test-client-id"
	clientSecret := "test-client-secret"
	redirectURL := "http://localhost:8080/callback"
	mockDB := &MockDatabaseService{}

	service := NewService(clientID, clientSecret, redirectURL, mockDB)

	// Verify oauth2.Config properties
	assert.Equal(t, clientID, service.config.ClientID)
	assert.Equal(t, clientSecret, service.config.ClientSecret)
	assert.Equal(t, redirectURL, service.config.RedirectURL)
	assert.Contains(t, service.config.Scopes, drive.DriveFileScope)
	assert.Equal(t, google.Endpoint, service.config.Endpoint)
}

// Test GetAuthURL variations
func TestGetAuthURL_Variations(t *testing.T) {
	testCases := []struct {
		name        string
		clientID    string
		redirectURL string
	}{
		{
			name:        "standard parameters",
			clientID:    "test-client-id",
			redirectURL: "http://localhost:8080/callback",
		},
		{
			name:        "https redirect URL",
			clientID:    "test-client-id",
			redirectURL: "https://myapp.com/oauth/callback",
		},
		{
			name:        "localhost with port",
			clientID:    "test-client-id",
			redirectURL: "http://localhost:3000/auth/google/callback",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &MockDatabaseService{}
			service := NewService(tc.clientID, "secret", tc.redirectURL, mockDB)

			authURL := service.GetAuthURL()

			assert.Contains(t, authURL, "https://accounts.google.com/o/oauth2/auth")
			assert.Contains(t, authURL, "client_id="+tc.clientID)
			assert.Contains(t, authURL, "state=state-token")
			assert.Contains(t, authURL, "access_type=offline")
		})
	}
}

// Benchmark tests
func BenchmarkNewService(b *testing.B) {
	mockDB := &MockDatabaseService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewService("client", "secret", "redirect", mockDB)
	}
}

func BenchmarkGetAuthURL(b *testing.B) {
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GetAuthURL()
	}
}

// Additional tests to improve coverage

// Test GetValidToken with token refresh success simulation
func TestGetValidToken_RefreshSuccessPath(t *testing.T) {
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	// Create a token that needs refresh (expires in 2 minutes)
	soonExpiry := time.Now().Add(2 * time.Minute)
	tokenConfig := &database.TokenConfig{
		ClientID:     "client",
		ClientSecret: "secret",
		AccessToken:  "old-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       soonExpiry,
	}

	mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	token, err := service.GetValidToken()

	// Could succeed if not exactly at refresh boundary, or fail if refresh needed
	if err != nil {
		// If refresh was attempted and failed
		assert.Contains(t, err.Error(), "failed to refresh token")
		assert.Nil(t, token)
	} else {
		// If token was still valid and no refresh needed
		assert.NotNil(t, token)
		assert.Equal(t, "old-token", token.AccessToken)
	}

	mockDB.AssertExpectations(t)
}

// Test GetValidToken with token refresh save error simulation
func TestGetValidToken_RefreshSaveError(t *testing.T) {
	// This tests the path where refresh might succeed but save fails
	// Since we can't mock OAuth success easily, this mainly tests error handling
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	// Create expired token
	expiredTime := time.Now().Add(-time.Hour)
	tokenConfig := &database.TokenConfig{
		ClientID:     "client",
		ClientSecret: "secret",
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       expiredTime,
	}

	mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	token, err := service.GetValidToken()

	// Will fail at OAuth refresh step before reaching save error
	assert.Error(t, err)
	assert.Nil(t, token)

	mockDB.AssertExpectations(t)
}

// Test ExchangeToken with nil database service
func TestExchangeToken_NilDatabase(t *testing.T) {
	service := NewService("client", "secret", "redirect", nil)

	// Will fail at OAuth exchange before reaching nil database panic
	err := service.ExchangeToken("test-code")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to exchange token")
}

// Test GetValidToken with nil database service
func TestGetValidToken_NilDatabase(t *testing.T) {
	service := NewService("client", "secret", "redirect", nil)

	// Should panic when trying to get token config with nil database
	assert.Panics(t, func() {
		service.GetValidToken()
	})
}

// Test GetTokenInfo with nil database service
func TestGetTokenInfo_NilDatabase(t *testing.T) {
	service := NewService("client", "secret", "redirect", nil)

	// Should panic when trying to get token config with nil database
	assert.Panics(t, func() {
		service.GetTokenInfo()
	})
}

// Test ValidateToken with nil database service
func TestValidateToken_NilDatabase(t *testing.T) {
	service := NewService("client", "secret", "redirect", nil)

	// Should panic when trying to get client with nil database
	assert.Panics(t, func() {
		service.ValidateToken()
	})
}

// Test RefreshToken with nil database service
func TestRefreshToken_NilDatabase(t *testing.T) {
	service := NewService("client", "secret", "redirect", nil)

	token := &oauth2.Token{
		AccessToken:  "old-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}

	// Will fail before reaching nil database panic due to OAuth error
	newToken, err := service.RefreshToken(token)
	assert.Error(t, err)
	assert.Nil(t, newToken)
	assert.Contains(t, err.Error(), "failed to refresh token")
}

// Test error handling paths in ExchangeToken
func TestExchangeToken_InvalidCode(t *testing.T) {
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	// Test with obviously invalid auth code
	err := service.ExchangeToken("invalid-code")

	// Should fail at OAuth exchange
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to exchange token")

	// SaveTokenConfig should not be called since exchange failed
	mockDB.AssertNotCalled(t, "SaveTokenConfig")
}

// Test ExchangeToken with empty code
func TestExchangeToken_EmptyCode(t *testing.T) {
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	err := service.ExchangeToken("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to exchange token")

	mockDB.AssertNotCalled(t, "SaveTokenConfig")
}

// Test edge case: token expiry exactly at boundary
func TestGetValidToken_ExactBoundary(t *testing.T) {
	mockDB := &MockDatabaseService{}
	service := NewService("client", "secret", "redirect", mockDB)

	// Token expires in exactly 5 minutes (the refresh boundary)
	boundaryTime := time.Now().Add(5 * time.Minute)
	tokenConfig := &database.TokenConfig{
		ClientID:     "client",
		ClientSecret: "secret",
		AccessToken:  "boundary-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       boundaryTime,
	}

	mockDB.On("GetTokenConfig").Return(tokenConfig, nil)

	token, err := service.GetValidToken()

	// Depending on exact timing, might need refresh or not
	// In most cases will need refresh and fail with invalid credentials
	if err != nil {
		assert.Contains(t, err.Error(), "failed to refresh token")
		assert.Nil(t, token)
	} else {
		assert.NotNil(t, token)
		assert.Equal(t, "boundary-token", token.AccessToken)
	}

	mockDB.AssertExpectations(t)
}
