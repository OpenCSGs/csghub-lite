package csghub

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GetTokenDetail returns the owner details for a token value.
func (c *Client) GetTokenDetail(ctx context.Context, tokenValue string) (*TokenDetail, error) {
	tokenValue = strings.TrimSpace(tokenValue)
	if tokenValue == "" {
		return nil, fmt.Errorf("token value is empty")
	}

	path := "/api/v1/token/" + url.PathEscape(tokenValue)
	var resp APIResponse[TokenDetail]

	anon := NewClient(c.baseURL, "")
	anon.httpClient = c.httpClient
	if err := anon.getJSON(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting token detail: %w", err)
	}

	return &resp.Data, nil
}

// GetUser returns details for a specific user.
func (c *Client) GetUser(ctx context.Context, username string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is empty")
	}

	path := "/api/v1/user/" + url.PathEscape(username)
	var resp APIResponse[User]
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting user %s: %w", username, err)
	}

	return &resp.Data, nil
}

// GetCurrentUser resolves the current user from the configured access token.
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	tokenValue := strings.TrimSpace(c.token)
	if tokenValue == "" {
		return nil, fmt.Errorf("missing access token")
	}

	detail, err := c.GetTokenDetail(ctx, tokenValue)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(detail.UserName)
	if username == "" {
		return nil, fmt.Errorf("token owner username is empty")
	}

	user, err := c.GetUser(ctx, username)
	if err != nil {
		return &User{
			Username: username,
			UUID:     strings.TrimSpace(detail.UserUUID),
		}, nil
	}

	if strings.TrimSpace(user.Username) == "" {
		user.Username = username
	}
	if strings.TrimSpace(user.UUID) == "" {
		user.UUID = strings.TrimSpace(detail.UserUUID)
	}

	return user, nil
}
