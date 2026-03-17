package dataset

import "time"

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
