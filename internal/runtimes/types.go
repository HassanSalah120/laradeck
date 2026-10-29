package runtimes

type ExtensionStatus struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
	Description string `json:"description,omitempty"`
}

type RuntimeInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        string            `json:"type"` // "standalone" or "herd"
	BinaryPath  string            `json:"binaryPath"`
	IniPath     string            `json:"iniPath"`
	Installed   bool              `json:"installed"`
	IsDefault   bool              `json:"isDefault"`
	DownloadURL string            `json:"downloadUrl,omitempty"`
	Extensions  []ExtensionStatus `json:"extensions,omitempty"`
}

type ComposerStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	InPath    bool   `json:"inPath"`
}
