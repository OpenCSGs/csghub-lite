package server

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/model"
	"github.com/opencsgs/csghub-lite/pkg/api"
)

const defaultModelUploadMemory = 32 << 20

// POST /api/models/upload -- import a local model archive, folder, or file set
func (s *Server) handleModelUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(defaultModelUploadMemory); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart upload: "+err.Error())
		return
	}
	defer r.MultipartForm.RemoveAll()

	form := r.MultipartForm
	files := form.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "at least one file is required")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(firstFormValue(form.Value, "mode")))
	if mode == "" {
		mode = "files"
	}
	overwrite := parseUploadBool(firstFormValue(form.Value, "overwrite"))
	paths := form.Value["paths"]
	modelID := strings.TrimSpace(firstFormValue(form.Value, "model"))
	if modelID == "" {
		modelID = deriveUploadModelID(files[0].Filename)
	}

	staging, err := os.MkdirTemp("", "csghub-model-upload-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "creating upload staging dir: "+err.Error())
		return
	}
	defer os.RemoveAll(staging)

	source := filepath.Join(staging, "files")
	if err := os.MkdirAll(source, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "creating upload dir: "+err.Error())
		return
	}

	kind := model.ImportSourceDirectory
	if mode == "archive" {
		if len(files) != 1 {
			writeError(w, http.StatusBadRequest, "archive upload requires exactly one file")
			return
		}
		relPath := safeUploadFileName(files[0].Filename)
		if relPath == "" {
			relPath = "model"
		}
		source = filepath.Join(staging, relPath)
		if err := saveUploadedFile(files[0], source); err != nil {
			writeError(w, http.StatusInternalServerError, "saving uploaded archive: "+err.Error())
			return
		}
		kind = model.ImportSourceArchive
	} else if mode == "directory" || mode == "files" {
		for i, header := range files {
			relPath := ""
			if i < len(paths) {
				relPath = paths[i]
			}
			if relPath == "" {
				relPath = header.Filename
			}
			cleanRel, err := cleanUploadRelativePath(relPath)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := saveUploadedFile(header, filepath.Join(source, filepath.FromSlash(cleanRel))); err != nil {
				writeError(w, http.StatusInternalServerError, "saving uploaded file: "+err.Error())
				return
			}
		}
	} else {
		writeError(w, http.StatusBadRequest, "unsupported upload mode")
		return
	}

	lm, err := s.manager.Import(model.ImportOptions{
		ModelID:   modelID,
		Source:    source,
		Kind:      kind,
		Overwrite: overwrite,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	lm, err = s.manager.GetWithFileEntries(lm.FullName())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, api.ModelUploadResponse{
		Status:  "success",
		Model:   lm.FullName(),
		Details: s.localModelInfo(lm),
		Files:   s.modelFileEntries(lm),
	})
}

func (s *Server) modelFileEntries(lm *model.LocalModel) []api.ModelFileEntry {
	files := make([]api.ModelFileEntry, 0, len(lm.FileEntries))
	for _, entry := range lm.FileEntries {
		files = append(files, api.ModelFileEntry{
			Path:        entry.Path,
			Size:        entry.Size,
			SHA256:      entry.SHA256,
			LFS:         entry.LFS,
			DownloadURL: buildModelFileDownloadURL(lm.Namespace, lm.Name, entry.Path),
		})
	}
	return files
}

func saveUploadedFile(header *multipart.FileHeader, target string) error {
	src, err := header.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Close()
}

func firstFormValue(values map[string][]string, key string) string {
	if list := values[key]; len(list) > 0 {
		return list[0]
	}
	return ""
}

func parseUploadBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func cleanUploadRelativePath(raw string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "/" || cleaned == "" || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid upload path %q", raw)
	}
	return strings.TrimPrefix(cleaned, "./"), nil
}

func safeUploadFileName(raw string) string {
	name := filepath.Base(strings.TrimSpace(raw))
	name = strings.TrimSpace(name)
	if name == "." || name == string(os.PathSeparator) {
		return ""
	}
	return name
}

func deriveUploadModelID(filename string) string {
	name := safeUploadFileName(filename)
	for _, suffix := range []string{".tar.gz", ".tgz", ".zip", ".tar", ".gguf", ".safetensors", ".bin"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}
	name = strings.Trim(name, " ._-")
	if name == "" {
		name = "uploaded-model"
	}
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	cleaned := strings.Trim(b.String(), "-._")
	if cleaned == "" {
		cleaned = "uploaded-model"
	}
	return "local/" + cleaned
}
