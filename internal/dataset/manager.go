package dataset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencsgs/csghub-lite/internal/config"
	"github.com/opencsgs/csghub-lite/internal/csghub"
)

type Manager struct {
	cfg    *config.Config
	client *csghub.Client
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:    cfg,
		client: csghub.NewClient(cfg.ServerURL, cfg.Token),
	}
}

func (m *Manager) Pull(ctx context.Context, datasetID string, progress csghub.SnapshotProgressFunc) (*LocalDataset, error) {
	namespace, name, err := csghub.ParseRepoID(datasetID)
	if err != nil {
		return nil, err
	}

	destDir := DatasetDir(m.cfg.DatasetDir, namespace, name)
	if err := EnsureDatasetDir(m.cfg.DatasetDir, namespace, name); err != nil {
		return nil, fmt.Errorf("creating dataset dir: %w", err)
	}

	info, err := m.client.GetDataset(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("fetching dataset info: %w", err)
	}

	downloadedFiles, err := m.client.DatasetSnapshotDownload(ctx, namespace, name, destDir, progress)
	if err != nil {
		return nil, fmt.Errorf("downloading dataset: %w", err)
	}

	var fileNames []string
	var totalSize int64
	for _, f := range downloadedFiles {
		fileNames = append(fileNames, f.Name)
		if f.Size > 0 {
			totalSize += f.Size
		} else {
			fi, err := os.Stat(filepath.Join(destDir, f.Path))
			if err == nil {
				totalSize += fi.Size()
			}
		}
	}

	ld := &LocalDataset{
		Namespace:    namespace,
		Name:         name,
		Size:         totalSize,
		Files:        fileNames,
		DownloadedAt: time.Now(),
		Description:  info.Description,
		License:      info.License,
	}

	if err := SaveManifest(m.cfg.DatasetDir, ld); err != nil {
		return nil, fmt.Errorf("saving manifest: %w", err)
	}

	return ld, nil
}

func (m *Manager) List() ([]*LocalDataset, error) {
	namespaces, err := ListNamespaces(m.cfg.DatasetDir)
	if err != nil {
		return nil, err
	}

	var datasets []*LocalDataset
	for _, ns := range namespaces {
		names, err := ListDatasetsInNamespace(m.cfg.DatasetDir, ns)
		if err != nil {
			continue
		}
		for _, name := range names {
			ld, err := LoadManifest(m.cfg.DatasetDir, ns, name)
			if err != nil {
				continue
			}
			datasets = append(datasets, ld)
		}
	}
	return datasets, nil
}

func (m *Manager) Get(datasetID string) (*LocalDataset, error) {
	namespace, name, err := csghub.ParseRepoID(datasetID)
	if err != nil {
		return nil, err
	}
	return LoadManifest(m.cfg.DatasetDir, namespace, name)
}

func (m *Manager) Remove(datasetID string) error {
	namespace, name, err := csghub.ParseRepoID(datasetID)
	if err != nil {
		return err
	}

	if _, err := LoadManifest(m.cfg.DatasetDir, namespace, name); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("dataset %q not found locally", datasetID)
		}
		return err
	}

	return RemoveDatasetDir(m.cfg.DatasetDir, namespace, name)
}

func (m *Manager) DatasetPath(datasetID string) (string, error) {
	namespace, name, err := csghub.ParseRepoID(datasetID)
	if err != nil {
		return "", err
	}
	dir := DatasetDir(m.cfg.DatasetDir, namespace, name)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("dataset %q not found locally", datasetID)
	}
	return dir, nil
}

func (m *Manager) Exists(datasetID string) bool {
	namespace, name, err := csghub.ParseRepoID(datasetID)
	if err != nil {
		return false
	}
	_, err = LoadManifest(m.cfg.DatasetDir, namespace, name)
	return err == nil
}

func (m *Manager) Client() *csghub.Client {
	return m.client
}
