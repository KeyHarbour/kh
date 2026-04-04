package khclient

import (
	"context"
	"net/http"
	"net/url"
)

// ListTeamMembers returns all team members for the organisation.
func (c *Client) ListTeamMembers(ctx context.Context) ([]TeamMember, error) {
	resp, err := c.do(ctx, http.MethodGet, "/license/team_members", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := expectStatus("list team members", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []TeamMember
	return out, decodeJSON(resp, &out)
}

// GetTeamMember returns a single team member by UUID.
func (c *Client) GetTeamMember(ctx context.Context, uuid string) (TeamMember, error) {
	if uuid == "" {
		return TeamMember{}, APIError{StatusCode: http.StatusBadRequest, Message: "team member uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/license/team_members/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return TeamMember{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get team member", resp, http.StatusOK); err != nil {
		return TeamMember{}, err
	}
	var out TeamMember
	if err := decodeJSON(resp, &out); err != nil {
		return TeamMember{}, err
	}
	if out.UUID == "" {
		out.UUID = uuid
	}
	return out, nil
}

// CreateTeamMember adds a team member.
func (c *Client) CreateTeamMember(ctx context.Context, req CreateTeamMemberRequest) error {
	body := struct {
		TeamMember CreateTeamMemberRequest `json:"team_member"`
	}{TeamMember: req}
	resp, err := c.do(ctx, http.MethodPost, "/license/team_members", nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("create team member", resp, http.StatusCreated)
}

// UpdateTeamMember updates an existing team member.
func (c *Client) UpdateTeamMember(ctx context.Context, uuid string, req UpdateTeamMemberRequest) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "team member uuid is required"}
	}
	body := struct {
		TeamMember UpdateTeamMemberRequest `json:"team_member"`
	}{TeamMember: req}
	resp, err := c.do(ctx, http.MethodPatch, "/license/team_members/"+url.PathEscape(uuid), nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update team member", resp, http.StatusAccepted)
}

// DeleteTeamMember removes a team member by UUID.
func (c *Client) DeleteTeamMember(ctx context.Context, uuid string) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "team member uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodDelete, "/license/team_members/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete team member", resp, http.StatusNoContent)
}
