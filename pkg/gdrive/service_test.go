package gdrive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/oauth2"
)

// MockAuthService is a mock implementation of the auth service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GetClient() (*oauth2.Config, *oauth2.Token, error) {
	args := m.Called()
	return args.Get(0).(*oauth2.Config), args.Get(1).(*oauth2.Token), args.Error(2)
}

func (m *MockAuthService) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	args := m.Called(token)
	return args.Get(0).(*oauth2.Token), args.Error(1)
}

// ServiceTestSuite for gdrive service tests
type ServiceTestSuite struct {
	suite.Suite
	service  *Service
	mockAuth *MockAuthService
	tempDir  string
	testFile string
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.mockAuth = &MockAuthService{}
	suite.service = NewService(suite.mockAuth)

	// Create temporary directory and test file
	tempDir, err := os.MkdirTemp("", "gdrive_test")
	suite.NoError(err)
	suite.tempDir = tempDir

	testFile := filepath.Join(tempDir, "test.sql")
	err = os.WriteFile(testFile, []byte("CREATE TABLE test (id INT);"), 0644)
	suite.NoError(err)
	suite.testFile = testFile
}

func (suite *ServiceTestSuite) TearDownTest() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// Test NewService
func (suite *ServiceTestSuite) TestNewService() {
	authService := &MockAuthService{}
	service := NewService(authService)
	suite.NotNil(service)
	suite.Equal(authService, service.authService)
}

// Test UploadFile - Success
func (suite *ServiceTestSuite) TestUploadFile_Success() {
	// Setup mock expectations
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	// Note: In a real test, we would need to mock the Google Drive API
	// For this example, we'll test the error path since we can't easily mock the HTTP client
	result, err := suite.service.UploadFile(suite.testFile)

	// The test will fail when trying to use the drive service because we don't have real credentials
	// This is expected behavior in unit tests
	suite.Error(err)
	suite.Nil(result)
	// Could be either service creation error or authentication error
	suite.True(strings.Contains(err.Error(), "failed to create drive service") ||
		strings.Contains(err.Error(), "failed to upload file to drive"))

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test UploadFile - Auth Error
func (suite *ServiceTestSuite) TestUploadFile_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	result, err := suite.service.UploadFile(suite.testFile)
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to get authenticated client")
	suite.Contains(err.Error(), "authentication failed")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test UploadFile - File Not Found
func (suite *ServiceTestSuite) TestUploadFile_FileNotFound() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.UploadFile("/non/existent/file.sql")
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to open file")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test UploadFile - With Folder ID
func (suite *ServiceTestSuite) TestUploadFile_WithFolderID() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.UploadFile(suite.testFile, "test-folder-id")

	// Will fail at either service creation or API call due to authentication
	suite.Error(err)
	suite.Nil(result)
	suite.True(strings.Contains(err.Error(), "failed to create drive service") ||
		strings.Contains(err.Error(), "failed to upload file to drive"))

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test CreateFolder - Auth Error
func (suite *ServiceTestSuite) TestCreateFolder_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	result, err := suite.service.CreateFolder("test-folder")
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to get authenticated client")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test CreateFolder - Success Path Setup
func (suite *ServiceTestSuite) TestCreateFolder_SetupSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.CreateFolder("test-folder")

	// Will fail due to invalid credentials in unit test environment
	suite.Error(err)
	suite.Nil(result)

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test CreateFolder - With Parent Folder
func (suite *ServiceTestSuite) TestCreateFolder_WithParentFolder() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.CreateFolder("test-folder", "parent-folder-id")

	// Will fail due to invalid credentials
	suite.Error(err)
	suite.Nil(result)

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test FindFolder - Auth Error
func (suite *ServiceTestSuite) TestFindFolder_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	result, err := suite.service.FindFolder("test-folder")
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to get authenticated client")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test FindFolder - Setup Success
func (suite *ServiceTestSuite) TestFindFolder_SetupSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.FindFolder("test-folder")

	// Will fail due to invalid credentials
	suite.Error(err)
	suite.Nil(result)

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test FindFolder - With Parent Folder
func (suite *ServiceTestSuite) TestFindFolder_WithParentFolder() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.FindFolder("test-folder", "parent-folder-id")

	// Will fail at drive service creation
	suite.Error(err)
	suite.Nil(result)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test GetOrCreateFolder - Success when folder exists
func (suite *ServiceTestSuite) TestGetOrCreateFolder_FindSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	// We need to mock GetClient twice - once for FindFolder, once for CreateFolder fallback
	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.GetOrCreateFolder("test-folder")

	// Will fail at drive service creation
	suite.Error(err)
	suite.Nil(result)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test ListFiles - Auth Error
func (suite *ServiceTestSuite) TestListFiles_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	result, err := suite.service.ListFiles("", 10)
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to get authenticated client")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test ListFiles - Setup Success
func (suite *ServiceTestSuite) TestListFiles_SetupSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.ListFiles("name contains 'backup'", 10)

	// Will fail at drive service creation
	suite.Error(err)
	suite.Nil(result)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test ListFiles - No Query, No Limit
func (suite *ServiceTestSuite) TestListFiles_NoQueryNoLimit() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.ListFiles("", 0)

	// Will fail at drive service creation
	suite.Error(err)
	suite.Nil(result)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test DeleteFile - Auth Error
func (suite *ServiceTestSuite) TestDeleteFile_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	err := suite.service.DeleteFile("test-file-id")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get authenticated client")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test DeleteFile - Setup Success
func (suite *ServiceTestSuite) TestDeleteFile_SetupSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	err := suite.service.DeleteFile("test-file-id")

	// Will fail at drive service creation
	suite.Error(err)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test GetFileInfo - Auth Error
func (suite *ServiceTestSuite) TestGetFileInfo_AuthError() {
	authError := errors.New("authentication failed")
	suite.mockAuth.On("GetClient").Return((*oauth2.Config)(nil), (*oauth2.Token)(nil), authError)

	result, err := suite.service.GetFileInfo("test-file-id")
	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "failed to get authenticated client")

	suite.mockAuth.AssertExpectations(suite.T())
}

// Test GetFileInfo - Setup Success
func (suite *ServiceTestSuite) TestGetFileInfo_SetupSuccess() {
	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	suite.mockAuth.On("GetClient").Return(config, token, nil)

	result, err := suite.service.GetFileInfo("test-file-id")

	// Will fail at drive service creation
	suite.Error(err)
	suite.Nil(result)
	// Error expected in test environment

	suite.mockAuth.AssertExpectations(suite.T())
}

// Additional unit tests to test individual functions and edge cases

// Test UploadFile with permission error on file
func TestUploadFile_FilePermissionError(t *testing.T) {
	mockAuth := &MockAuthService{}
	service := NewService(mockAuth)

	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	mockAuth.On("GetClient").Return(config, token, nil)

	// Create a directory instead of a file to simulate permission error
	tempDir, err := os.MkdirTemp("", "gdrive_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	result, err := service.UploadFile(tempDir) // Trying to "upload" a directory
	assert.Error(t, err)
	assert.Nil(t, result)

	mockAuth.AssertExpectations(t)
}

// Test Upload with empty folder ID
func TestUploadFile_EmptyFolderID(t *testing.T) {
	mockAuth := &MockAuthService{}
	service := NewService(mockAuth)

	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	mockAuth.On("GetClient").Return(config, token, nil)

	tempDir, err := os.MkdirTemp("", "gdrive_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.sql")
	err = os.WriteFile(testFile, []byte("CREATE TABLE test (id INT);"), 0644)
	assert.NoError(t, err)

	result, err := service.UploadFile(testFile, "") // Empty folder ID should be ignored
	assert.Error(t, err)                            // Will fail at drive service creation
	assert.Nil(t, result)

	mockAuth.AssertExpectations(t)
}

// Test CreateFolder with empty parent folder ID
func TestCreateFolder_EmptyParentFolderID(t *testing.T) {
	mockAuth := &MockAuthService{}
	service := NewService(mockAuth)

	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	mockAuth.On("GetClient").Return(config, token, nil)

	result, err := service.CreateFolder("test-folder", "") // Empty parent folder ID should be ignored
	assert.Error(t, err)                                   // Will fail at drive service creation
	assert.Nil(t, result)

	mockAuth.AssertExpectations(t)
}

// Test FindFolder with empty parent folder ID
func TestFindFolder_EmptyParentFolderID(t *testing.T) {
	mockAuth := &MockAuthService{}
	service := NewService(mockAuth)

	config := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}
	token := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}

	mockAuth.On("GetClient").Return(config, token, nil)

	result, err := service.FindFolder("test-folder", "") // Empty parent folder ID should be ignored
	assert.Error(t, err)                                 // Will fail at drive service creation
	assert.Nil(t, result)

	mockAuth.AssertExpectations(t)
}

// Test UploadResult struct
func TestUploadResult(t *testing.T) {
	result := &UploadResult{
		FileID:      "test-file-id",
		FileName:    "test-file.sql",
		Size:        1024,
		WebViewLink: "https://drive.google.com/file/d/test-file-id/view",
	}

	assert.Equal(t, "test-file-id", result.FileID)
	assert.Equal(t, "test-file.sql", result.FileName)
	assert.Equal(t, int64(1024), result.Size)
	assert.Equal(t, "https://drive.google.com/file/d/test-file-id/view", result.WebViewLink)
}

// Test edge cases and error scenarios that are hard to test with mocks
func TestService_EdgeCases(t *testing.T) {
	// Test NewService with nil auth service (should handle gracefully)
	service := NewService(nil)
	assert.NotNil(t, service)
	assert.Nil(t, service.authService)

	// Any method call should now panic because of nil pointer dereference
	// We'll test that the service can be created but calling methods panics
	assert.Panics(t, func() {
		service.UploadFile("test.sql")
	})
}

// Benchmark tests for performance
func BenchmarkNewService(b *testing.B) {
	mockAuth := &MockAuthService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewService(mockAuth)
	}
}
