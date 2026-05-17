package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coffeenights/conure/pkg/api"
)

// SetCredential creates or rotates an org credential. The server upserts by
// (org, name): posting a name that already exists replaces its material in
// place, otherwise it inserts. Returns the metadata view (never the secret).
func (c *Client) SetCredential(ctx context.Context, orgID string, req api.CreateCredentialRequest) (*api.Credential, error) {
	data, err := c.post(ctx, fmt.Sprintf("/credentials/%s/c", orgID), req)
	if err != nil {
		return nil, err
	}
	var cred api.Credential
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, err
	}
	return &cred, nil
}

// ListCredentials returns the org's credentials as metadata only — the API
// never sends secret material, so there is nothing to mask client-side.
func (c *Client) ListCredentials(ctx context.Context, orgID string) ([]api.Credential, error) {
	data, err := c.get(ctx, fmt.Sprintf("/credentials/%s/c", orgID))
	if err != nil {
		return nil, err
	}
	var creds []api.Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func (c *Client) DeleteCredential(ctx context.Context, orgID, name string) error {
	_, _, err := c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/credentials/%s/c/%s", orgID, name), nil)
	return err
}
