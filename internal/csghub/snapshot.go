package csghub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	return c.downloadSnapshot(ctx, "models", namespace, name, destDir, files, progress)
}

const maxConcurrentDownloads = 3

// DatasetSnapshotDownload downloads all files in a dataset repository.
// It uses the /csg/ endpoints which work without authentication for public datasets.
// Up to 3 files are downloaded concurrently.
func (c *Client) DatasetSnapshotDownload(ctx context.Context, namespace, name, destDir string, progress SnapshotProgressFunc) ([]RepoFile, error) {
	files, err := c.GetDatasetTree(ctx, namespace, name)
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
		return nil, fmt.Errorf("no files found in dataset %s/%s", namespace, name)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, maxConcurrentDownloads)
	var mu sync.Mutex
	var firstErr error

	var wg sync.WaitGroup
	for i, f := range downloadFiles {
		wg.Add(1)
		go func(idx int, file RepoFile) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			if firstErr != nil {
				mu.Unlock()
				return
			}
			mu.Unlock()

			destPath := filepath.Join(destDir, file.Path)

			fileProgress := func(downloaded, total int64) {
				if progress != nil {
					progress(SnapshotProgress{
						FileName:       file.Name,
						FileIndex:      idx,
						TotalFiles:     len(downloadFiles),
						BytesCompleted: downloaded,
						BytesTotal:     total,
					})
				}
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("creating directory for %s: %w", file.Path, err)
					cancel()
				}
				mu.Unlock()
				return
			}

			var existingSize int64
			if info, err := os.Stat(destPath); err == nil {
				existingSize = info.Size()
			}

			downloadURL := fmt.Sprintf("%s/csg/datasets/%s/%s/resolve/main/%s",
				c.baseURL, namespace, name, file.Path)

			if err := c.downloadFromURL(ctx, downloadURL, destPath, existingSize, 0, fileProgress); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("downloading %s: %w", file.Path, err)
					cancel()
				}
				mu.Unlock()
			}
		}(i, f)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return downloadFiles, nil
}

func (c *Client) downloadSnapshot(ctx context.Context, repoType, namespace, name, destDir string, files []RepoFile, progress SnapshotProgressFunc) ([]RepoFile, error) {
	var downloadFiles []RepoFile
	for _, f := range files {
		if f.Type == "file" {
			downloadFiles = append(downloadFiles, f)
		}
	}

	if len(downloadFiles) == 0 {
		return nil, fmt.Errorf("no files found in %s/%s", namespace, name)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, maxConcurrentDownloads)
	var mu sync.Mutex
	var firstErr error

	var wg sync.WaitGroup
	for i, f := range downloadFiles {
		wg.Add(1)
		go func(idx int, file RepoFile) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			if firstErr != nil {
				mu.Unlock()
				return
			}
			mu.Unlock()

			destPath := filepath.Join(destDir, file.Path)

			fileProgress := func(downloaded, total int64) {
				if progress != nil {
					progress(SnapshotProgress{
						FileName:       file.Name,
						FileIndex:      idx,
						TotalFiles:     len(downloadFiles),
						BytesCompleted: downloaded,
						BytesTotal:     total,
					})
				}
			}

			if err := c.DownloadRepoFile(ctx, repoType, namespace, name, file.Path, destPath, file.LFS, file.Size, file.LFSSHA256, fileProgress); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("downloading %s: %w", file.Path, err)
					cancel()
				}
				mu.Unlock()
			}
		}(i, f)
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return downloadFiles, nil
}

// ParseModelID splits a model identifier like "namespace/name" into parts.
func ParseModelID(modelID string) (namespace, name string, err error) {
	return ParseRepoID(modelID)
}

// ParseRepoID splits a repository identifier like "namespace/name" into parts.
func ParseRepoID(id string) (namespace, name string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid ID %q: expected format namespace/name", id)
	}
	return parts[0], parts[1], nil
}
