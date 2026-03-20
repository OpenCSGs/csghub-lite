package dataset

import (
	"os"
	"path/filepath"
	"time"
)

type LocalDataset struct {
	Namespace    string    `json:"namespace"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Files        []string  `json:"files"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Description  string    `json:"description,omitempty"`
	License      string    `json:"license,omitempty"`
}

func (d *LocalDataset) FullName() string {
	return d.Namespace + "/" + d.Name
}

type FileEntry struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	ModifiedAt time.Time `json:"modified_at"`
}

func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}
