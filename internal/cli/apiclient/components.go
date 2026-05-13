package apiclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coffeenights/conure/pkg/api"
)

func (c *Client) ListComponentDefinitions(ctx context.Context, orgID string) ([]api.ComponentDefinition, error) {
	data, err := c.get(ctx, fmt.Sprintf("/organizations/%s/component-definitions", orgID))
	if err != nil {
		return nil, err
	}
	var resp api.ComponentDefinitionListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Definitions, nil
}

func (c *Client) ListAppComponents(ctx context.Context, orgID, appID string) ([]api.Component, error) {
	data, err := c.get(ctx, fmt.Sprintf("/organizations/%s/a/%s/c", orgID, appID))
	if err != nil {
		return nil, err
	}
	var resp api.ComponentListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Components, nil
}

func (c *Client) CreateComponent(ctx context.Context, orgID, appID, name, typeName, environment string) (*api.CreateComponentResponse, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/c", orgID, appID),
		api.CreateComponentRequest{
			Name:        name,
			Type:        typeName,
			Environment: environment,
			Values:      map[string]interface{}{},
		},
	)
	if err != nil {
		return nil, err
	}
	var resp api.CreateComponentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetComponentInEnv(ctx context.Context, orgID, appID, env, componentID string) (*api.ComponentInEnvResponse, error) {
	data, err := c.get(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s", orgID, appID, env, componentID),
	)
	if err != nil {
		return nil, err
	}
	var resp api.ComponentInEnvResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListComponentPods(ctx context.Context, orgID, appID, env, componentID string) ([]api.Pod, error) {
	data, err := c.get(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/pods", orgID, appID, env, componentID),
	)
	if err != nil {
		return nil, err
	}
	var resp api.ComponentPodsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Pods, nil
}
