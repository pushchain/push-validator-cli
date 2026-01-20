package update

import "time"

// Release represents a GitHub release
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"` // Changelog/release notes
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset (binary archive)
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// CheckResult holds the result of an update check
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	Release         *Release
}

// UpdateOptions configures the update behavior
type UpdateOptions struct {
	Force      bool   // Skip version comparison
	SkipVerify bool   // Skip checksum verification (not recommended)
	Version    string // Specific version to install (empty = latest)
}
