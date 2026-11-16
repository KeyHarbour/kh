package khclient

import (
	"encoding/json"
	"time"
)

type Statefile struct {
	UUID        string    `json:"uuid"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at"`
	Environment string    `json:"environment,omitempty"`
}

type StatefileCreatedResponse struct {
	Status string `json:"status"`
}

type Project struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Environments []string `json:"environment_names,omitempty"`
}

// UnmarshalJSON accepts the environment list as "environments", which
// KeyHarbourApi returns, or "environment_names", which the OpenAPI spec
// documents (KeyHarbour/KeyHarbourApi#19). Output keeps "environment_names".
func (p *Project) UnmarshalJSON(b []byte) error {
	type plain Project
	var aux struct {
		plain
		APIEnvironments []string `json:"environments"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*p = Project(aux.plain)
	if len(p.Environments) == 0 {
		p.Environments = aux.APIEnvironments
	}
	return nil
}

type CreateProjectRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	EnvironmentNames []string `json:"environment_names,omitempty"`
}

// UpdateProjectRequest only sends the fields that are set.
type UpdateProjectRequest struct {
	Name             string   `json:"name,omitempty"`
	EnvironmentNames []string `json:"environment_names,omitempty"`
}

type Workspace struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type KeyValue struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	RawValue    []byte  `json:"-"`
	ExpiresAt   *string `json:"expires_at"`
	Private     bool    `json:"private"`
	OneTimeOnly bool    `json:"one_time_only,omitempty"`
	Environment string  `json:"environment,omitempty"`
}

type CreateKeyValueRequest struct {
	Key             string  `json:"key"`
	Payload         string  `json:"value"`
	PayloadFromFile bool    `json:"-"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
	Private         bool    `json:"private,omitempty"`
	OneTimeOnly     bool    `json:"one_time_only,omitempty"`
}

type UpdateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Application struct {
	UUID        string   `json:"uuid,omitempty"`
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name"`
	Owner       string   `json:"owner"`
	Vendor      string   `json:"vendor"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Tier        string   `json:"tier,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
	Status      string   `json:"status,omitempty"`
}

type CreateApplicationRequest struct {
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name"`
	Owner       string   `json:"owner"`
	Vendor      string   `json:"vendor"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Tier        string   `json:"tier,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
}

type UpdateApplicationRequest struct {
	Name        string   `json:"name,omitempty"`
	ShortName   string   `json:"short_name,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	Vendor      string   `json:"vendor,omitempty"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Tier        string   `json:"tier,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
	Status      string   `json:"status,omitempty"`
}

type Instance struct {
	UUID        string   `json:"uuid,omitempty"`
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name"`
	Owner       string   `json:"owner,omitempty"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
	Status      string   `json:"status,omitempty"`
}

type CreateInstanceRequest struct {
	Name        string   `json:"name"`
	ShortName   string   `json:"short_name"`
	Owner       string   `json:"owner,omitempty"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
}

type UpdateInstanceRequest struct {
	Name        string   `json:"name,omitempty"`
	ShortName   string   `json:"short_name,omitempty"`
	Owner       string   `json:"owner,omitempty"`
	RenewalDate string   `json:"renewal_date,omitempty"`
	Seats       *int     `json:"seats,omitempty"`
	UnitCost    *float64 `json:"unit_cost,omitempty"`
	Status      string   `json:"status,omitempty"`
}

type Licensee struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name,omitempty"`
	Email  string `json:"email,omitempty"`
	Status string `json:"status,omitempty"`
}

type CreateLicenseeRequest struct {
	UUID string `json:"uuid"`
}

type UpdateLicenseeRequest struct {
	UUID   string `json:"uuid"`
	Status string `json:"status,omitempty"`
}

type TeamMember struct {
	UUID        string  `json:"uuid"`
	ManagerUUID *string `json:"manager_uuid"`
}

type CreateTeamMemberRequest struct {
	UUID string `json:"uuid"`
}

type UpdateTeamMemberRequest struct {
	UUID        string `json:"uuid"`
	ManagerUUID string `json:"manager_uuid"`
}

type UpdateKeyValueRequest struct {
	Payload         string  `json:"value"`
	PayloadFromFile bool    `json:"-"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
	Private         *bool   `json:"private,omitempty"`
	OneTimeOnly     *bool   `json:"one_time_only,omitempty"`
}
