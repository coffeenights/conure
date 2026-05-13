package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coffeenights/conure/pkg/api"
)

func (c *Client) ListApps(ctx context.Context, orgID string) ([]api.Application, error) {
	data, err := c.get(ctx, fmt.Sprintf("/organizations/%s/a", orgID))
	if err != nil {
		return nil, err
	}
	var resp api.ApplicationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Applications, nil
}

func (c *Client) GetApp(ctx context.Context, orgID, appID string) (*api.Application, error) {
	data, err := c.get(ctx, fmt.Sprintf("/organizations/%s/a/%s", orgID, appID))
	if err != nil {
		return nil, err
	}
	var app api.Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (c *Client) CreateApp(ctx context.Context, orgID, name, description string) (*api.Application, error) {
	data, err := c.post(ctx, fmt.Sprintf("/organizations/%s/a", orgID), api.CreateApplicationRequest{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	var app api.Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (c *Client) CreateEnvironment(ctx context.Context, orgID, appID, name string) error {
	_, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e", orgID, appID),
		api.CreateEnvironmentRequest{Name: name},
	)
	return err
}

// DeleteEnvironment removes an environment by name. The server cascades to
// the environment's revisions/components, so the caller is expected to
// confirm before invoking.
func (c *Client) DeleteEnvironment(ctx context.Context, orgID, appID, name string) error {
	_, _, err := c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s", orgID, appID, name), nil)
	return err
}
