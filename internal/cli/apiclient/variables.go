package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coffeenights/conure/pkg/api"
)

// VariableScope identifies which path the variables endpoints should use.
// The server routes are keyed by scope: a different URL for org-, env-, and
// component-scoped variables. Keeping scope as a single value (rather than
// three sibling methods on Client) lets command code switch on it without
// duplicating list/create/delete plumbing.
type VariableScope struct {
	OrgID         string
	ApplicationID string // required for env + component scopes
	EnvironmentID string // required for env + component scopes
	ComponentID   string // required for component scope
	// AllScopes flips list URLs to the merged /allscopes variant for the
	// env and component tiers. Ignored for org-only scope (nothing above it
	// to merge) and ignored for write operations (those always target a
	// concrete tier).
	AllScopes bool
}

// path returns the /variables/... URL for this scope. Returns an empty
// string if the scope is missing fields it needs (caller treats that as a
// programming error — commands should validate before calling).
func (s VariableScope) path() string {
	if s.OrgID == "" {
		return ""
	}
	if s.ComponentID != "" {
		if s.ApplicationID == "" || s.EnvironmentID == "" {
			return ""
		}
		base := fmt.Sprintf("/variables/%s/%s/e/%s/c/%s",
			s.OrgID, s.ApplicationID, s.EnvironmentID, s.ComponentID)
		if s.AllScopes {
			return base + "/allscopes"
		}
		return base
	}
	if s.EnvironmentID != "" {
		if s.ApplicationID == "" {
			return ""
		}
		base := fmt.Sprintf("/variables/%s/%s/e/%s",
			s.OrgID, s.ApplicationID, s.EnvironmentID)
		if s.AllScopes {
			return base + "/allscopes"
		}
		return base
	}
	return fmt.Sprintf("/variables/%s", s.OrgID)
}

func (c *Client) ListVariables(ctx context.Context, scope VariableScope) ([]api.Variable, error) {
	p := scope.path()
	if p == "" {
		return nil, fmt.Errorf("invalid variable scope")
	}
	data, err := c.get(ctx, p)
	if err != nil {
		return nil, err
	}
	// Server emits a bare JSON array, not a wrapped object.
	var vars []api.Variable
	if err := json.Unmarshal(data, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

func (c *Client) CreateVariable(ctx context.Context, scope VariableScope, name, value string, isSecret bool) (*api.Variable, error) {
	p := scope.path()
	if p == "" {
		return nil, fmt.Errorf("invalid variable scope")
	}
	data, err := c.post(ctx, p, api.CreateVariableRequest{
		Name:        name,
		Value:       value,
		IsEncrypted: isSecret,
	})
	if err != nil {
		return nil, err
	}
	var v api.Variable
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// DeleteVariable removes a variable by ID. The server scopes deletion by
// orgID; appID/envID/componentID in the scope are ignored.
func (c *Client) DeleteVariable(ctx context.Context, orgID, variableID string) error {
	_, _, err := c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/variables/%s/%s", orgID, variableID), nil)
	return err
}
