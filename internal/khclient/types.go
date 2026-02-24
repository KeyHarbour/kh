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
	Environments []string `json:"environments,omitempty"`
}

type Workspace struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type KeyValue struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at"`
	Private   bool    `json:"private"`
}

type CreateKeyValueRequest struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	Private   bool    `json:"private,omitempty"`
}

type UpdateKeyValueRequest struct {
	Value     string  `json:"value"`
	ExpiresAt *string `json:"expires_at,omitempty"`
	Private   *bool   `json:"private,omitempty"`
}
