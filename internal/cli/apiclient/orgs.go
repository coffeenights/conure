package apiclient

import (
	"context"
	"encoding/json"

	"github.com/coffeenights/conure/pkg/api"
)

func (c *Client) ListOrganizations(ctx context.Context) ([]api.Organization, error) {
	data, err := c.get(ctx, "/organizations/")
	if err != nil {
		return nil, err
	}
	var resp api.OrganizationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

// CreateOrganization creates an org owned by the authenticated account and
// returns its metadata. The server flattens the org into the response body
// (id/name/status/account_id/created_at), so it unmarshals straight into
// api.Organization — no envelope, matching CreateApp's shape.
func (c *Client) CreateOrganization(ctx context.Context, name string) (*api.Organization, error) {
	data, err := c.post(ctx, "/organizations/", api.CreateOrganizationRequest{Name: name})
	if err != nil {
		return nil, err
	}
	var org api.Organization
	if err := json.Unmarshal(data, &org); err != nil {
		return nil, err
	}
	return &org, nil
}
