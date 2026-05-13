package apiclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coffeenights/conure/pkg/api"
)

func (c *Client) ListRevisions(ctx context.Context, orgID, appID, env, componentID string) ([]api.ComponentRevision, error) {
	data, err := c.get(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions", orgID, appID, env, componentID),
	)
	if err != nil {
		return nil, err
	}
	var resp api.ComponentRevisionListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Revisions, nil
}

func (c *Client) CreateRevision(ctx context.Context, orgID, appID, env, componentID string, values map[string]interface{}, comment string) (*api.ComponentRevision, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions", orgID, appID, env, componentID),
		api.CreateRevisionRequest{Values: values, Comment: comment},
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (c *Client) UpdateRevision(ctx context.Context, orgID, appID, env, componentID, revID string, values map[string]interface{}, comment string) (*api.ComponentRevision, error) {
	data, err := c.put(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions/%s", orgID, appID, env, componentID, revID),
		api.UpdateRevisionRequest{Values: values, Comment: comment},
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (c *Client) RestartComponent(ctx context.Context, orgID, appID, env, componentID string) (*api.ComponentRevision, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/restart", orgID, appID, env, componentID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (c *Client) DeployLatestDraft(ctx context.Context, orgID, appID, env, componentID string) (*api.ComponentRevision, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/deploy", orgID, appID, env, componentID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (c *Client) DeployRevision(ctx context.Context, orgID, appID, env, componentID, revID string) (*api.ComponentRevision, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions/%s/deploy", orgID, appID, env, componentID, revID),
		nil,
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}

func (c *Client) Promote(ctx context.Context, orgID, appID, componentID, from, to string) (*api.ComponentRevision, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/c/%s/promote", orgID, appID, componentID),
		api.PromoteRequest{From: from, To: to},
	)
	if err != nil {
		return nil, err
	}
	var rev api.ComponentRevision
	if err := json.Unmarshal(data, &rev); err != nil {
		return nil, err
	}
	return &rev, nil
}
