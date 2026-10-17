package models

// ReceivedUpload is what PUT /uploads/{token} answers.
type ReceivedUpload struct {
	AssetURL string `json:"assetUrl"`
	FileID   string `json:"fileId"`
	Size     int    `json:"size"`
}
