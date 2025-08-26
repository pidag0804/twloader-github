// twloader-tool/optimizer/types.go
package optimizer

type OptimizationItem struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Category   string `json:"category"`
	FileURL    string `json:"fileURL"`
	ImageURL   string `json:"imageURL"`
	TargetFile string `json:"targetFile"`
}

type UpdateItem struct {
	Path         string `json:"path"`
	SizeExpected int64  `json:"sizeExpected"`
	URL          string `json:"url"`
	BackupURL    string `json:"backupUrl"`
	Name         string `json:"-"`
	RelativePath string `json:"-"`
}

type UpdateCheckResponse struct {
	OK           bool         `json:"ok"`
	UpdateNeeded bool         `json:"updateNeeded"`
	Items        []UpdateItem `json:"items"`
	Error        string       `json:"error,omitempty"`
}

type ApplyUpdatesRequest struct {
	Items []UpdateItem `json:"items"`
}

type ApplyUpdatesResponse struct {
	OK        bool           `json:"ok"`
	Updated   []string       `json:"updated"`
	Failed    []FailedUpdate `json:"failed"`
	Message   string         `json:"message"`
	Error     string         `json:"error,omitempty"`
	NeedAdmin bool           `json:"needAdmin,omitempty"`
}

type FailedUpdate struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}
