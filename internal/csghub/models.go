package csghub

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ListModels returns a paginated list of models.
func (c *Client) ListModels(ctx context.Context, params ModelListParams) ([]Model, int, error) {
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
	if params.Framework != "" {
		q.Set("framework", params.Framework)
	}
	if params.TagCategory != "" {
		q.Set("tag_category", params.TagCategory)
	}
	if params.TagName != "" {
		q.Set("tag_name", params.TagName)
	}

	path := "/api/v1/models"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp ListResponse[Model]
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, 0, fmt.Errorf("listing models: %w", err)
	}
	return resp.Data, resp.Total, nil
}

// GetModel returns details for a specific model.
func (c *Client) GetModel(ctx context.Context, namespace, name string) (*Model, error) {
	path := fmt.Sprintf("/api/v1/models/%s/%s", namespace, name)

	var resp APIResponse[Model]
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting model %s/%s: %w", namespace, name, err)
	}
	return &resp.Data, nil
}

// GetModelTree returns the file tree for a model repository.
func (c *Client) GetModelTree(ctx context.Context, namespace, name string) ([]RepoFile, error) {
	path := fmt.Sprintf("/api/v1/models/%s/%s/tree", namespace, name)

	var resp APIResponse[[]RepoFile]
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting file tree for %s/%s: %w", namespace, name, err)
	}
	return resp.Data, nil
}

// SearchModels searches for models by keyword.
func (c *Client) SearchModels(ctx context.Context, query string, page, perPage int) ([]Model, int, error) {
	return c.ListModels(ctx, ModelListParams{
		Search:  query,
		Page:    page,
		PerPage: perPage,
	})
}
