package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/opencsgs/csghub-lite/internal/csghub"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

// GET /api/datasets -- list local datasets
func (s *Server) handleDatasetTags(w http.ResponseWriter, r *http.Request) {
	datasets, err := s.datasetManager.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var infos []api.DatasetInfo
	for _, d := range datasets {
		infos = append(infos, api.DatasetInfo{
			Name:       d.FullName(),
			Dataset:    d.FullName(),
			Size:       d.Size,
			Files:      len(d.Files),
			ModifiedAt: d.DownloadedAt,
		})
	}

	writeJSON(w, http.StatusOK, api.DatasetTagsResponse{Datasets: infos})
}

// POST /api/datasets/show -- dataset details
func (s *Server) handleDatasetShow(w http.ResponseWriter, r *http.Request) {
	var req api.DatasetShowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ld, err := s.datasetManager.Get(req.Dataset)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("dataset %q not found", req.Dataset))
		return
	}

	writeJSON(w, http.StatusOK, api.DatasetShowResponse{
		Details: api.DatasetInfo{
			Name:       ld.FullName(),
			Dataset:    ld.FullName(),
			Size:       ld.Size,
			Files:      len(ld.Files),
			ModifiedAt: ld.DownloadedAt,
		},
		Files: ld.Files,
	})
}

// POST /api/datasets/pull -- download a dataset
func (s *Server) handleDatasetPull(w http.ResponseWriter, r *http.Request) {
	var req api.DatasetPullRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var mu sync.Mutex
	safeSSE := func(v interface{}) {
		mu.Lock()
		writeSSE(w, v)
		mu.Unlock()
	}

	safeSSE(api.DatasetPullResponse{Status: "pulling " + req.Dataset})

	progress := func(p csghub.SnapshotProgress) {
		safeSSE(api.DatasetPullResponse{
			Status:    fmt.Sprintf("downloading %s", p.FileName),
			Digest:    p.FileName,
			Total:     p.BytesTotal,
			Completed: p.BytesCompleted,
		})
	}

	_, err := s.datasetManager.Pull(r.Context(), req.Dataset, progress)
	if err != nil {
		log.Printf("pull dataset %s failed: %v", req.Dataset, err)
		safeSSE(api.DatasetPullResponse{Status: "error: " + err.Error()})
		return
	}

	safeSSE(api.DatasetPullResponse{Status: "success"})
}

// POST /api/datasets/files -- browse files in a dataset directory
func (s *Server) handleDatasetFiles(w http.ResponseWriter, r *http.Request) {
	var req api.DatasetFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entries, err := s.datasetManager.ListFiles(req.Dataset, req.Path)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("cannot list files: %v", err))
		return
	}

	var apiEntries []api.DatasetFileEntry
	for _, e := range entries {
		apiEntries = append(apiEntries, api.DatasetFileEntry{
			Name:       e.Name,
			Size:       e.Size,
			IsDir:      e.IsDir,
			ModifiedAt: e.ModifiedAt,
		})
	}

	writeJSON(w, http.StatusOK, api.DatasetFilesResponse{
		Dataset: req.Dataset,
		Path:    req.Path,
		Entries: apiEntries,
	})
}

// DELETE /api/datasets/delete -- remove a dataset
func (s *Server) handleDatasetDelete(w http.ResponseWriter, r *http.Request) {
	var req api.DatasetDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.datasetManager.Remove(req.Dataset); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
