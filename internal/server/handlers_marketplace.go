package server

import (
	"net/http"
	"strconv"

	"github.com/opencsgs/csghub-lite/internal/csghub"
)

// GET /api/marketplace/models -- proxy to CSGHub Hub model listing
func (s *Server) handleMarketplaceModels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	per, _ := strconv.Atoi(q.Get("per"))
	if page <= 0 {
		page = 1
	}
	if per <= 0 {
		per = 16
	}

	client := csghub.NewClient(s.cfg.ServerURL, s.cfg.Token)
	models, total, err := client.ListModels(r.Context(), csghub.ModelListParams{
		Search:  q.Get("search"),
		Sort:    q.Get("sort"),
		Page:    page,
		PerPage: per,
		Source:  q.Get("source"),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  models,
		"total": total,
	})
}

// GET /api/marketplace/datasets -- proxy to CSGHub Hub dataset listing
func (s *Server) handleMarketplaceDatasets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	per, _ := strconv.Atoi(q.Get("per"))
	if page <= 0 {
		page = 1
	}
	if per <= 0 {
		per = 16
	}

	client := csghub.NewClient(s.cfg.ServerURL, s.cfg.Token)
	datasets, total, err := client.ListDatasets(r.Context(), csghub.DatasetListParams{
		Search:  q.Get("search"),
		Sort:    q.Get("sort"),
		Page:    page,
		PerPage: per,
		Source:  q.Get("source"),
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  datasets,
		"total": total,
	})
}
