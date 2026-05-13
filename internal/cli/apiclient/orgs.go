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
