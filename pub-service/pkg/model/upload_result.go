package model

type UploadResult struct {
	File   string `json:"file"`
	Status string `json:"status"`
	Key    string `json:"key,omitempty"`
	Error  string `json:"error,omitempty"`
}

type UploadResponse struct {
	Status     string         `json:"status"`
	TotalFiles int            `json:"total_files"`
	Uploaded   int            `json:"uploaded"`
	Failed     int            `json:"failed"`
	Results    []UploadResult `json:"results"`
	Duration   string         `json:"duration"`
}
