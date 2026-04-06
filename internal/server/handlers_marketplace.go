package server

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/opencsgs/csghub-lite/internal/csghub"
	"github.com/opencsgs/csghub-lite/internal/ggufpick"
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

	requestedFramework := normalizeMarketplaceFramework(q.Get("framework"))
	listParams := csghub.ModelListParams{
		Search:  q.Get("search"),
		Sort:    q.Get("sort"),
		Page:    page,
		PerPage: per,
		Source:  q.Get("source"),
	}
	if requestedFramework != "" {
		listParams.TagCategory = "framework"
		listParams.TagName = requestedFramework
	}

	client := csghub.NewClient(s.cfg.ServerURL, s.cfg.Token)
	models, total, err := client.ListModels(r.Context(), listParams)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if requestedFramework != "" && !marketplaceModelsMatchFramework(models, requestedFramework) {
		models, total, err = listMarketplaceModelsWithFrameworkFallback(r.Context(), client, listParams, requestedFramework)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  models,
		"total": total,
	})
}

type marketplaceModelQuantization struct {
	Name        string `json:"name"`
	FileCount   int    `json:"file_count"`
	ExamplePath string `json:"example_path"`
}

type marketplaceModelDetailResponse struct {
	Details       *csghub.Model                  `json:"details"`
	Quantizations []marketplaceModelQuantization `json:"quantizations,omitempty"`
}

// GET /api/marketplace/models/{namespace}/{name} -- proxy to CSGHub model detail
func (s *Server) handleMarketplaceModelDetail(w http.ResponseWriter, r *http.Request) {
	namespace := r.PathValue("namespace")
	name := r.PathValue("name")
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, "missing namespace or name")
		return
	}

	client := csghub.NewClient(s.cfg.ServerURL, s.cfg.Token)
	details, err := client.GetModel(r.Context(), namespace, name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var quantizations []marketplaceModelQuantization
	if marketplaceModelFormat(details.Tags) == "gguf" {
		if files, err := client.GetModelTree(r.Context(), namespace, name); err == nil {
			quantizations = summarizeMarketplaceQuantizations(files)
		}
	}

	writeJSON(w, http.StatusOK, marketplaceModelDetailResponse{
		Details:       details,
		Quantizations: quantizations,
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

func marketplaceModelFormat(tags []csghub.Tag) string {
	for _, tag := range tags {
		if tag.Category != "framework" {
			continue
		}
		name := normalizeMarketplaceFramework(tag.Name)
		switch name {
		case "gguf", "safetensors":
			return name
		}
	}
	return ""
}

func normalizeMarketplaceFramework(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gguf":
		return "gguf"
	case "safetensors":
		return "safetensors"
	default:
		return ""
	}
}

func marketplaceModelHasFramework(tags []csghub.Tag, framework string) bool {
	framework = normalizeMarketplaceFramework(framework)
	if framework == "" {
		return true
	}
	for _, tag := range tags {
		if tag.Category != "framework" {
			continue
		}
		if normalizeMarketplaceFramework(tag.Name) == framework {
			return true
		}
	}
	return false
}

func marketplaceModelsMatchFramework(models []csghub.Model, framework string) bool {
	for _, model := range models {
		if !marketplaceModelHasFramework(model.Tags, framework) {
			return false
		}
	}
	return true
}

func listMarketplaceModelsWithFrameworkFallback(
	ctx context.Context,
	client *csghub.Client,
	params csghub.ModelListParams,
	framework string,
) ([]csghub.Model, int, error) {
	const maxFallbackPages = 3

	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PerPage <= 0 {
		params.PerPage = 16
	}

	offset := (params.Page - 1) * params.PerPage
	upstreamPerPage := params.PerPage * 8
	if upstreamPerPage < 64 {
		upstreamPerPage = 64
	}
	if upstreamPerPage > 100 {
		upstreamPerPage = 100
	}

	items := make([]csghub.Model, 0, params.PerPage)
	matchedCount := 0

	for upstreamPage := 1; upstreamPage <= maxFallbackPages; upstreamPage++ {
		batch, upstreamTotal, err := client.ListModels(ctx, csghub.ModelListParams{
			Search:      params.Search,
			Sort:        params.Sort,
			Page:        upstreamPage,
			PerPage:     upstreamPerPage,
			Source:      params.Source,
			TagCategory: params.TagCategory,
			TagName:     params.TagName,
		})
		if err != nil {
			if len(items) > 0 || matchedCount > 0 {
				return items, approximateMarketplaceFilteredTotal(offset, len(items), matchedCount), nil
			}
			return nil, 0, err
		}

		for _, model := range batch {
			if !marketplaceModelHasFramework(model.Tags, framework) {
				continue
			}
			if matchedCount >= offset && len(items) < params.PerPage {
				items = append(items, model)
			}
			matchedCount++
		}

		exhausted := len(batch) == 0 || upstreamPage*upstreamPerPage >= upstreamTotal
		if len(items) >= params.PerPage {
			if exhausted {
				return items, matchedCount, nil
			}
			// Upstream doesn't honor the filter reliably, so expose the current page
			// and keep the next page navigable without claiming a fake exact total.
			return items, offset + len(items) + 1, nil
		}
		if exhausted {
			return items, matchedCount, nil
		}
	}

	return items, approximateMarketplaceFilteredTotal(offset, len(items), matchedCount), nil
}

func approximateMarketplaceFilteredTotal(offset, itemCount, matchedCount int) int {
	total := matchedCount
	minimumNextPageTotal := offset + itemCount + 1
	if total < minimumNextPageTotal {
		total = minimumNextPageTotal
	}
	return total
}

func summarizeMarketplaceQuantizations(files []csghub.RepoFile) []marketplaceModelQuantization {
	type agg struct {
		item marketplaceModelQuantization
		rank int
	}

	byName := make(map[string]*agg)
	for _, file := range files {
		path := marketplaceRepoFilePath(file)
		if !ggufpick.IsWeightGGUF(path) {
			continue
		}
		label := ggufpick.QuantLabelFromRepoPath(path)
		if label == "" {
			continue
		}
		entry, ok := byName[label]
		if !ok {
			entry = &agg{
				item: marketplaceModelQuantization{
					Name:        label,
					ExamplePath: path,
				},
				rank: ggufpick.QuantRankFromRepoPath(path),
			}
			byName[label] = entry
		}
		entry.item.FileCount++
		if entry.item.ExamplePath == "" || strings.Compare(path, entry.item.ExamplePath) < 0 {
			entry.item.ExamplePath = path
		}
	}

	out := make([]marketplaceModelQuantization, 0, len(byName))
	keys := make([]string, 0, len(byName))
	for key := range byName {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := byName[keys[i]]
		right := byName[keys[j]]
		if left.rank != right.rank {
			return left.rank > right.rank
		}
		return left.item.Name < right.item.Name
	})
	for _, key := range keys {
		out = append(out, byName[key].item)
	}
	return out
}

func marketplaceRepoFilePath(file csghub.RepoFile) string {
	if strings.TrimSpace(file.Path) != "" {
		return file.Path
	}
	return file.Name
}
