package gdrive

// File represents a simplified Google Drive file
type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	CreatedTime string `json:"created_time"`
	WebViewLink string `json:"web_view_link"`
}
