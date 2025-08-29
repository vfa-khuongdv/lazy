package gdrive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/vfa-khuongdv/go-backup-drive/internal/auth"
)

// Service handles Google Drive operations
type Service struct {
	authService *auth.Service
}

// NewService creates a new Google Drive service
func NewService(authService *auth.Service) *Service {
	return &Service{
		authService: authService,
	}
}

// UploadResult contains information about the uploaded file
type UploadResult struct {
	FileID      string `json:"file_id"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	WebViewLink string `json:"web_view_link"`
}

// UploadFile uploads a file to Google Drive
func (s *Service) UploadFile(filePath string, folderID ...string) (*UploadResult, error) {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileName := filepath.Base(filePath)

	// Create drive file metadata
	driveFile := &drive.File{
		Name:        fileName,
		Description: fmt.Sprintf("Database backup created on %s", time.Now().Format("2006-01-02 15:04:05")),
	}

	// Set parent folder if provided
	if len(folderID) > 0 && folderID[0] != "" {
		driveFile.Parents = []string{folderID[0]}
	}

	// Upload the file
	res, err := driveService.Files.Create(driveFile).Media(file, googleapi.ContentType("application/sql")).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to drive: %w", err)
	}

	return &UploadResult{
		FileID:      res.Id,
		FileName:    res.Name,
		Size:        fileInfo.Size(),
		WebViewLink: res.WebViewLink,
	}, nil
}

// CreateFolder creates a folder in Google Drive
func (s *Service) CreateFolder(name string, parentFolderID ...string) (*drive.File, error) {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
	}

	// Set parent folder if provided
	if len(parentFolderID) > 0 && parentFolderID[0] != "" {
		folder.Parents = []string{parentFolderID[0]}
	}

	res, err := driveService.Files.Create(folder).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	return res, nil
}

// FindFolder finds a folder by name
func (s *Service) FindFolder(name string, parentFolderID ...string) (*drive.File, error) {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", name)

	// Add parent folder constraint if provided
	if len(parentFolderID) > 0 && parentFolderID[0] != "" {
		query = fmt.Sprintf("%s and '%s' in parents", query, parentFolderID[0])
	}

	res, err := driveService.Files.List().Q(query).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to search for folder: %w", err)
	}

	if len(res.Files) == 0 {
		return nil, fmt.Errorf("folder '%s' not found", name)
	}

	return res.Files[0], nil
}

// GetOrCreateFolder gets an existing folder or creates a new one
func (s *Service) GetOrCreateFolder(name string, parentFolderID ...string) (*drive.File, error) {
	// Try to find existing folder first
	folder, err := s.FindFolder(name, parentFolderID...)
	if err == nil {
		return folder, nil
	}

	// Create new folder if not found
	return s.CreateFolder(name, parentFolderID...)
}

// ListFiles lists files in Google Drive with optional query
func (s *Service) ListFiles(query string, maxResults int64) ([]*drive.File, error) {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	call := driveService.Files.List().
		Fields("files(id,name,size,createdTime,modifiedTime,webViewLink)").
		OrderBy("createdTime desc")

	if query != "" {
		call = call.Q(query)
	}

	if maxResults > 0 {
		call = call.PageSize(maxResults)
	}

	res, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return res.Files, nil
}

// DeleteFile deletes a file from Google Drive
func (s *Service) DeleteFile(fileID string) error {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create drive service: %w", err)
	}

	err = driveService.Files.Delete(fileID).Do()
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFileInfo gets information about a file
func (s *Service) GetFileInfo(fileID string) (*drive.File, error) {
	config, token, err := s.authService.GetClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get authenticated client: %w", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service: %w", err)
	}

	file, err := driveService.Files.Get(fileID).
		Fields("id,name,size,createdTime,modifiedTime,webViewLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return file, nil
}
