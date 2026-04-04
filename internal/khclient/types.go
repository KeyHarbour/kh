package khclient

import "time"

type StateMeta struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	Module    string    `json:"module"`
	Workspace string    `json:"workspace"`
	Lineage   string    `json:"lineage"`
	Serial    int       `json:"serial"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
}

type ListStatesRequest struct {
	Project   string `json:"project,omitempty"`
	Module    string `json:"module,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

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

type Workspace struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type KeyValue struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	ExpiresAt   *string `json:"expires_at"`
	Private     bool    `json:"private"`
	Environment string  `json:"environment,omitempty"`
}

type CreateKeyValueRequest struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	Private   bool    `json:"private,omitempty"`
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
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	Private   *bool   `json:"private,omitempty"`
}
