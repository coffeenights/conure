package apiclient

import (
	"context"
	"encoding/json"

	"github.com/coffeenights/conure/pkg/api"
)

// SystemInfo fetches cluster platform + Kubernetes version. Used by the
// deploy command to pick the right build target when the developer is on a
// different architecture than the cluster (M-series Mac → amd64 cluster is
// the canonical case).
func (c *Client) SystemInfo(ctx context.Context) (*api.SystemInfo, error) {
	data, err := c.get(ctx, "/system/info")
	if err != nil {
		return nil, err
	}
	var info api.SystemInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
