package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/coffeenights/conure/pkg/api"
)

// TriggerBuild creates a Build for the (component, env) pair. The server
// branches on req.BuildLocation:
//   - "local": records the already-pushed image and deploys synchronously
//     (response is the succeeded Build).
//   - "remote": creates a BuildKit Job and returns 202 with the Build in
//     pending/building state; caller polls GetBuild until terminal.
func (c *Client) TriggerBuild(ctx context.Context, orgID, appID, env, componentID string, req api.TriggerBuildRequest) (*api.Build, error) {
	data, err := c.post(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/builds", orgID, appID, env, componentID),
		req,
	)
	if err != nil {
		return nil, err
	}
	var b api.Build
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (c *Client) ListBuilds(ctx context.Context, orgID, appID, env, componentID string) ([]api.Build, error) {
	data, err := c.get(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/builds", orgID, appID, env, componentID),
	)
	if err != nil {
		return nil, err
	}
	var resp api.BuildListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Builds, nil
}

func (c *Client) GetBuild(ctx context.Context, orgID, appID, env, componentID, buildID string) (*api.Build, error) {
	data, err := c.get(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/builds/%s", orgID, appID, env, componentID, buildID),
	)
	if err != nil {
		return nil, err
	}
	var b api.Build
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// StreamBuildLogs returns the live log body for a remote build's Job pod.
// Caller MUST close. `follow=true` keeps the connection open until the pod
// exits.
func (c *Client) StreamBuildLogs(ctx context.Context, orgID, appID, env, componentID, buildID string, follow bool) (io.ReadCloser, error) {
	q := url.Values{}
	if follow {
		q.Set("follow", "true")
	}
	return c.Stream(ctx,
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/builds/%s/logs", orgID, appID, env, componentID, buildID),
		q,
	)
}
