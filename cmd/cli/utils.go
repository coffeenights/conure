package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coffeenights/conure/pkg/api"
)

// detectComponentName picks a friendly name from (in order):
//
//  1. the repo segment of `git config --get remote.origin.url`
//  2. basename(cwd)
//
// Returns the empty string only if cwd cannot be read.
func detectComponentName() string {
	if name := repoNameFromGitOrigin(); name != "" {
		return name
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(cwd)
}

func repoNameFromGitOrigin() string {
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return ""
	}
	// Strip trailing .git, then take the last path segment. Handles both
	// https://host/owner/repo(.git) and git@host:owner/repo(.git).
	url = strings.TrimSuffix(url, ".git")
	for _, sep := range []string{"/", ":"} {
		if i := strings.LastIndex(url, sep); i >= 0 {
			url = url[i+1:]
		}
	}
	return url
}

func gitOriginURL() string {
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitCurrentBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" { // detached
		return ""
	}
	return branch
}

func cwdBasename() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(cwd)
}

// ---- API call helpers (typed wrappers) ----------------------------------

func listOrgs(client *apiClient) ([]api.Organization, error) {
	data, err := client.get("/organizations/")
	if err != nil {
		return nil, err
	}
	var resp api.OrganizationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Organizations, nil
}

func listApps(client *apiClient, orgID string) ([]api.Application, error) {
	data, err := client.get(fmt.Sprintf("/organizations/%s/a", orgID))
	if err != nil {
		return nil, err
	}
	var resp api.ApplicationListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Applications, nil
}

func getApp(client *apiClient, orgID, appID string) (*api.Application, error) {
	data, err := client.get(fmt.Sprintf("/organizations/%s/a/%s", orgID, appID))
	if err != nil {
		return nil, err
	}
	var app api.Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func createApp(client *apiClient, orgID, name, description string) (*api.Application, error) {
	data, err := client.post(fmt.Sprintf("/organizations/%s/a", orgID), api.CreateApplicationRequest{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	var app api.Application
	if err := json.Unmarshal(data, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func createEnv(client *apiClient, orgID, appID, name string) error {
	_, err := client.post(
		fmt.Sprintf("/organizations/%s/a/%s/e", orgID, appID),
		api.CreateEnvironmentRequest{Name: name},
	)
	return err
}

func listComponentDefinitions(client *apiClient, orgID string) ([]api.ComponentDefinition, error) {
	data, err := client.get(fmt.Sprintf("/organizations/%s/component-definitions", orgID))
	if err != nil {
		return nil, err
	}
	var resp api.ComponentDefinitionListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Definitions, nil
}

func listAppComponents(client *apiClient, orgID, appID string) ([]api.Component, error) {
	data, err := client.get(fmt.Sprintf("/organizations/%s/a/%s/c", orgID, appID))
	if err != nil {
		return nil, err
	}
	var resp api.ComponentListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Components, nil
}

func createComponent(client *apiClient, orgID, appID, name, typeName, environment string) (*api.CreateComponentResponse, error) {
	data, err := client.post(
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

func deployLatestDraft(client *apiClient, orgID, appID, env, componentID string) (*api.ComponentRevision, error) {
	data, err := client.post(
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

func deployRevision(client *apiClient, orgID, appID, env, componentID, revID string) (*api.ComponentRevision, error) {
	data, err := client.post(
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

func getComponentInEnv(client *apiClient, orgID, appID, env, componentID string) (*api.ComponentInEnvResponse, error) {
	data, err := client.get(
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

func listRevisions(client *apiClient, orgID, appID, env, componentID string) ([]api.ComponentRevision, error) {
	data, err := client.get(
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

func createRevision(client *apiClient, orgID, appID, env, componentID string, values map[string]interface{}) (*api.ComponentRevision, error) {
	data, err := client.post(
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions", orgID, appID, env, componentID),
		api.CreateRevisionRequest{Values: values},
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

func updateRevision(client *apiClient, orgID, appID, env, componentID, revID string, values map[string]interface{}) (*api.ComponentRevision, error) {
	data, err := client.put(
		fmt.Sprintf("/organizations/%s/a/%s/e/%s/c/%s/revisions/%s", orgID, appID, env, componentID, revID),
		api.UpdateRevisionRequest{Values: values},
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

func listComponentPods(client *apiClient, orgID, appID, env, componentID string) ([]api.Pod, error) {
	data, err := client.get(
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

func promoteComponent(client *apiClient, orgID, appID, componentID, from, to string) (*api.ComponentRevision, error) {
	data, err := client.post(
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
