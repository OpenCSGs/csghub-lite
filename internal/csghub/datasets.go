package csghub

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
)

// ListDatasets returns a paginated list of datasets.
func (c *Client) ListDatasets(ctx context.Context, params DatasetListParams) ([]Dataset, int, error) {
	q := url.Values{}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	if params.Sort != "" {
		q.Set("sort", params.Sort)
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		q.Set("per", strconv.Itoa(params.PerPage))
	}
	if params.Source != "" {
		q.Set("source", params.Source)
	}

	apiPath := "/api/v1/datasets"
	if encoded := q.Encode(); encoded != "" {
		apiPath += "?" + encoded
	}

	var resp ListResponse[Dataset]
	if err := c.getJSON(ctx, apiPath, &resp); err != nil {
		return nil, 0, fmt.Errorf("listing datasets: %w", err)
	}
	return resp.Data, resp.Total, nil
}

// GetDataset returns details for a specific dataset.
func (c *Client) GetDataset(ctx context.Context, namespace, name string) (*Dataset, error) {
	apiPath := fmt.Sprintf("/api/v1/datasets/%s/%s", namespace, name)

	var resp APIResponse[Dataset]
	if err := c.getJSON(ctx, apiPath, &resp); err != nil {
		return nil, fmt.Errorf("getting dataset %s/%s: %w", namespace, name, err)
	}
	return &resp.Data, nil
}

// GetDatasetTree returns the file list for a dataset repository.
// It uses the /csg/api/ endpoint which works without authentication for public datasets,
// falling back to /api/v1/ if a token is available.
func (c *Client) GetDatasetTree(ctx context.Context, namespace, name string) ([]RepoFile, error) {
	csgPath := fmt.Sprintf("/csg/api/datasets/%s/%s/revision/main", namespace, name)
	var info RepoInfoResponse
	if err := c.getJSON(ctx, csgPath, &info); err != nil {
		return nil, fmt.Errorf("getting file list for dataset %s/%s: %w", namespace, name, err)
	}

	var files []RepoFile
	for _, s := range info.Siblings {
		files = append(files, RepoFile{
			Name: path.Base(s.RFilename),
			Path: s.RFilename,
			Type: "file",
		})
	}
	return files, nil
}

// SearchDatasets searches for datasets by keyword.
func (c *Client) SearchDatasets(ctx context.Context, query string, page, perPage int) ([]Dataset, int, error) {
	return c.ListDatasets(ctx, DatasetListParams{
		Search:  query,
		Page:    page,
		PerPage: perPage,
	})
}
