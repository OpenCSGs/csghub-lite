package model

import (
	"context"
	"fmt"
	"os"
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

// Pull downloads a model from CSGHub.
func (m *Manager) Pull(ctx context.Context, modelID string, progress csghub.SnapshotProgressFunc) (*LocalModel, error) {
	namespace, name, err := csghub.ParseModelID(modelID)
	if err != nil {
		return nil, err
	}

	destDir := ModelDir(m.cfg.ModelDir, namespace, name)
	if err := EnsureModelDir(m.cfg.ModelDir, namespace, name); err != nil {
		return nil, fmt.Errorf("creating model dir: %w", err)
	}

	info, err := m.client.GetModel(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("fetching model info: %w", err)
	}

	downloadedFiles, err := m.client.SnapshotDownload(ctx, namespace, name, destDir, progress)
	if err != nil {
		return nil, fmt.Errorf("downloading model: %w", err)
	}

	var fileNames []string
	var totalSize int64
	for _, f := range downloadedFiles {
		fileNames = append(fileNames, f.Name)
		totalSize += f.Size
	}

	lm := &LocalModel{
		Namespace:    namespace,
		Name:         name,
		Format:       DetectFormat(fileNames),
		Size:         totalSize,
		Files:        fileNames,
		DownloadedAt: time.Now(),
		Description:  info.Description,
		License:      info.License,
	}

	if err := SaveManifest(m.cfg.ModelDir, lm); err != nil {
		return nil, fmt.Errorf("saving manifest: %w", err)
	}

	return lm, nil
}

// List returns all locally downloaded models.
func (m *Manager) List() ([]*LocalModel, error) {
	namespaces, err := ListNamespaces(m.cfg.ModelDir)
	if err != nil {
		return nil, err
	}

	var models []*LocalModel
	for _, ns := range namespaces {
		names, err := ListModelsInNamespace(m.cfg.ModelDir, ns)
		if err != nil {
			continue
		}
		for _, name := range names {
			lm, err := LoadManifest(m.cfg.ModelDir, ns, name)
			if err != nil {
				continue
			}
			models = append(models, lm)
		}
	}
	return models, nil
}

// Get returns a locally downloaded model by ID.
func (m *Manager) Get(modelID string) (*LocalModel, error) {
	namespace, name, err := csghub.ParseModelID(modelID)
	if err != nil {
		return nil, err
	}
	return LoadManifest(m.cfg.ModelDir, namespace, name)
}

// Remove deletes a locally downloaded model.
func (m *Manager) Remove(modelID string) error {
	namespace, name, err := csghub.ParseModelID(modelID)
	if err != nil {
		return err
	}

	if _, err := LoadManifest(m.cfg.ModelDir, namespace, name); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("model %q not found locally", modelID)
		}
		return err
	}

	return RemoveModelDir(m.cfg.ModelDir, namespace, name)
}

// ModelPath returns the directory path for a model.
func (m *Manager) ModelPath(modelID string) (string, error) {
	namespace, name, err := csghub.ParseModelID(modelID)
	if err != nil {
		return "", err
	}
	dir := ModelDir(m.cfg.ModelDir, namespace, name)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("model %q not found locally", modelID)
	}
	return dir, nil
}

// Exists checks if a model is downloaded locally.
func (m *Manager) Exists(modelID string) bool {
	namespace, name, err := csghub.ParseModelID(modelID)
	if err != nil {
		return false
	}
	_, err = LoadManifest(m.cfg.ModelDir, namespace, name)
	return err == nil
}

// Client returns the underlying CSGHub client.
func (m *Manager) Client() *csghub.Client {
	return m.client
}
