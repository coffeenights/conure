package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coffeenights/conure/pkg/api"
)

// ListUsers returns every user. Admin-only on the server.
func (c *Client) ListUsers(ctx context.Context) ([]api.User, error) {
	data, err := c.get(ctx, "/users/")
	if err != nil {
		return nil, err
	}
	var resp api.UserListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Users, nil
}

func (c *Client) GetUser(ctx context.Context, id string) (*api.User, error) {
	data, err := c.get(ctx, "/users/"+id)
	if err != nil {
		return nil, err
	}
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.User, error) {
	data, err := c.post(ctx, "/users/", req)
	if err != nil {
		return nil, err
	}
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) UpdateUser(ctx context.Context, id string, req api.UpdateUserRequest) (*api.User, error) {
	data, _, err := c.do(ctx, http.MethodPatch, "/users/"+id, req)
	if err != nil {
		return nil, err
	}
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	_, _, err := c.do(ctx, http.MethodDelete, "/users/"+id, nil)
	return err
}

func (c *Client) ResetUserPassword(ctx context.Context, id string, password string) (string, error) {
	body := api.ResetPasswordRequest{Password: password}
	data, err := c.post(ctx, fmt.Sprintf("/users/%s/reset-password", id), body)
	if err != nil {
		return "", err
	}
	var resp api.ResetPasswordResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Password, nil
}

// GetMe returns the currently authenticated user. Wraps /auth/me.
func (c *Client) GetMe(ctx context.Context) (*api.User, error) {
	data, err := c.get(ctx, "/auth/me")
	if err != nil {
		return nil, err
	}
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) UpdateMe(ctx context.Context, email string) (*api.User, error) {
	req := api.UpdateMeRequest{Email: &email}
	data, _, err := c.do(ctx, http.MethodPatch, "/auth/me", req)
	if err != nil {
		return nil, err
	}
	var u api.User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (c *Client) ChangePassword(ctx context.Context, oldPwd, newPwd string) error {
	req := api.ChangePasswordRequest{
		OldPassword: oldPwd,
		Password:    newPwd,
		Password2:   newPwd,
	}
	_, _, err := c.do(ctx, http.MethodPatch, "/auth/change-password", req)
	return err
}
