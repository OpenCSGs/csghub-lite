package model

import "time"

type Format string

const (
	FormatGGUF        Format = "gguf"
	FormatSafeTensors Format = "safetensors"
	FormatUnknown     Format = "unknown"
)

type LocalModel struct {
	Namespace    string           `json:"namespace"`
	Name         string           `json:"name"`
	Format       Format           `json:"format"`
	Size         int64            `json:"size"`
	Files        []string         `json:"files"`
	FileEntries  []LocalModelFile `json:"file_entries,omitempty"`
	DownloadedAt time.Time        `json:"downloaded_at"`
	Description  string           `json:"description,omitempty"`
	License      string           `json:"license,omitempty"`
	PipelineTag  string           `json:"pipeline_tag,omitempty"`
}

type LocalModelFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	LFS    bool   `json:"lfs,omitempty"`
}

func (m *LocalModel) FullName() string {
	return m.Namespace + "/" + m.Name
}
