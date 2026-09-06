package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type WorkspaceInput struct {
	Name     string `json:"name"`
	JobRole  string `json:"job_role,omitempty"`
	TeamSize string `json:"team_size,omitempty"`
	Language string `json:"language"`
}

func (c *Client) CreateWorkspace(ctx context.Context, in WorkspaceInput) (*Workspace, json.RawMessage, error) {
	var out Workspace
	raw, err := c.post(ctx, "/workspace", in, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, json.RawMessage, error) {
	var out []Workspace
	raw, err := c.get(ctx, "/workspace", nil, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

type ProjectInput struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// BusinessGoals is ordered by priority (highest first).
	BusinessGoals    []string `json:"business_goals,omitempty"`
	ProductType      string   `json:"product_type,omitempty"`
	TargetAudience   []string `json:"target_audience,omitempty"`
	ExcludedAudience []string `json:"excluded_audience,omitempty"`
	ProductStage     string   `json:"product_stage,omitempty"`
}

func (c *Client) CreateProject(ctx context.Context, in ProjectInput) (*Project, json.RawMessage, error) {
	var out Project
	raw, err := c.post(ctx, "/project", in, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) ListProjects(ctx context.Context, workspaceID string) ([]Project, json.RawMessage, error) {
	q := url.Values{}
	if workspaceID != "" {
		q.Set("workspace_id", workspaceID)
	}
	var out []Project
	raw, err := c.get(ctx, "/project", q, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

func (c *Client) ListSpecs(ctx context.Context, projectID string) ([]Spec, json.RawMessage, error) {
	var out []Spec
	raw, err := c.get(ctx, "/spec/", url.Values{"project_id": {projectID}}, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

func (c *Client) GetSpec(ctx context.Context, id string) (*Spec, json.RawMessage, error) {
	var out Spec
	raw, err := c.get(ctx, "/spec/"+id, nil, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) LockSpec(ctx context.Context, id string) (*Spec, json.RawMessage, error) {
	var out Spec
	raw, err := c.patch(ctx, "/spec/"+id, map[string]bool{"locked": true}, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) ListRequests(ctx context.Context, projectID string) ([]Request, json.RawMessage, error) {
	var out []Request
	raw, err := c.get(ctx, "/request/", url.Values{"project_id": {projectID}}, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

func (c *Client) GetRequest(ctx context.Context, id string) (*Request, json.RawMessage, error) {
	var out Request
	raw, err := c.get(ctx, "/request/"+id, nil, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) RegenerateRequest(ctx context.Context, id, expectation string) (*Request, json.RawMessage, error) {
	var out Request
	raw, err := c.post(ctx, "/request/"+id+"/regenerate", map[string]string{"expectation": expectation}, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

type Answer struct {
	SolutionPathID string      `json:"solution_path_id"`
	Selections     []Selection `json:"selections"`
}

type Selection struct {
	Label  string `json:"label"`
	Custom bool   `json:"custom"`
}

func (c *Client) DraftRequest(ctx context.Context, id string, answers []Answer) (*Request, json.RawMessage, error) {
	var out Request
	raw, err := c.post(ctx, "/request/"+id+"/draft", map[string]any{"answers": answers}, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

type CaptureInput struct {
	ProjectID   string `json:"project_id"`
	Description string `json:"description"`
}

func (c *Client) CreateCapture(ctx context.Context, in CaptureInput) (*Capture, json.RawMessage, error) {
	var out Capture
	raw, err := c.post(ctx, "/capture/", in, &out)
	if err != nil {
		return nil, nil, err
	}
	return &out, raw, nil
}

func (c *Client) ListCaptures(ctx context.Context, projectID string) ([]Capture, json.RawMessage, error) {
	var out []Capture
	raw, err := c.get(ctx, "/capture/", url.Values{"project_id": {projectID}}, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

func (c *Client) ListSources(ctx context.Context, projectID string) ([]Source, json.RawMessage, error) {
	var out []Source
	raw, err := c.get(ctx, "/source", url.Values{"project_id": {projectID}}, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, raw, nil
}

func (c *Client) IndexSource(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodPost, "/source/"+id+"/index", nil, nil, nil, true)
	return err
}
