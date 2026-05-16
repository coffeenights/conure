package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

// SetComponentDefinition creates or overrides the org's definition for a
// (type, engine). The server upserts: posting for a key the org already has
// updates it in place (and un-hides a tombstone), otherwise it inserts a new
// org-owned override that shadows the shipped default for this org only. It
// returns the resulting definition.
func (c *Client) SetComponentDefinition(ctx context.Context, orgID string, req api.ComponentDefinitionRequest) (*api.ComponentDefinition, error) {
	data, err := c.post(ctx, fmt.Sprintf("/organizations/%s/component-definitions", orgID), req)
	if err != nil {
		return nil, err
	}
	var def api.ComponentDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// HideComponentDefinition writes a tombstone for a (type, engine) in the org,
// suppressing the inherited shipped default. Reversible via
// DeleteComponentDefinition on the returned row's id.
func (c *Client) HideComponentDefinition(ctx context.Context, orgID string, req api.HideComponentDefinitionRequest) (*api.ComponentDefinition, error) {
	data, err := c.post(ctx, fmt.Sprintf("/organizations/%s/component-definitions/hide", orgID), req)
	if err != nil {
		return nil, err
	}
	var def api.ComponentDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// DeleteComponentDefinition removes an org-owned row by id. Deleting an
// override restores the inherited default; deleting a tombstone un-hides it.
// Shipped defaults (source "default") are shared and cannot be deleted here —
// hide them instead.
func (c *Client) DeleteComponentDefinition(ctx context.Context, orgID, definitionID string) error {
	_, _, err := c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/organizations/%s/component-definitions/%s", orgID, definitionID), nil)
	return err
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

// CreateComponent posts a new component to the API. engine is optional —
// pass "" when the type has a single ComponentDefinition and the API can
// infer it; pass "timoni"/"helm" when multiple implementations of the same
// type are registered and the caller wants a specific one.
func (c *Client) CreateComponent(ctx context.Context, orgID, appID, name, typeName, engine, environment string) (*api.CreateComponentResponse, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/c", orgID, appID),
		api.CreateComponentRequest{
			Name:        name,
			Type:        typeName,
			Engine:      engine,
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
