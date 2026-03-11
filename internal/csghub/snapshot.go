package csghub

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// SnapshotProgress reports progress for a multi-file download.
type SnapshotProgress struct {
	FileName       string
	FileIndex      int
	TotalFiles     int
	BytesCompleted int64
	BytesTotal     int64
}

// SnapshotProgressFunc is called for each file progress update.
type SnapshotProgressFunc func(SnapshotProgress)

// SnapshotDownload downloads all files in a model repository, similar to
// pycsghub's snapshot_download.
func (c *Client) SnapshotDownload(ctx context.Context, namespace, name, destDir string, progress SnapshotProgressFunc) ([]RepoFile, error) {
	files, err := c.GetModelTree(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("fetching file tree: %w", err)
	}

	var downloadFiles []RepoFile
	for _, f := range files {
		if f.Type == "file" {
			downloadFiles = append(downloadFiles, f)
		}
	}

	if len(downloadFiles) == 0 {
		return nil, fmt.Errorf("no files found in %s/%s", namespace, name)
	}

	for i, f := range downloadFiles {
		destPath := filepath.Join(destDir, f.Path)

		fileProgress := func(downloaded, total int64) {
			if progress != nil {
				progress(SnapshotProgress{
					FileName:       f.Name,
					FileIndex:      i,
					TotalFiles:     len(downloadFiles),
					BytesCompleted: downloaded,
					BytesTotal:     total,
				})
			}
		}

		if err := c.DownloadFile(ctx, namespace, name, f.Path, destPath, f.LFS, f.Size, f.LFSSHA256, fileProgress); err != nil {
			return nil, fmt.Errorf("downloading %s: %w", f.Path, err)
		}
	}

	return downloadFiles, nil
}

// ParseModelID splits a model identifier like "namespace/name" into parts.
func ParseModelID(modelID string) (namespace, name string, err error) {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid model ID %q: expected format namespace/name", modelID)
	}
	return parts[0], parts[1], nil
}
