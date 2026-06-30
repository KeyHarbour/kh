package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"kh/internal/khclient"
)

const (
	kvBackendAuto   = "auto"
	kvBackendRemote = "remote"
	kvBackendFile   = "file"
)

type kvStore interface {
	ListKeyValues(ctx context.Context, workspace string) ([]khclient.KeyValue, error)
	GetKeyValue(ctx context.Context, workspace, key string) (khclient.KeyValue, error)
	CreateKeyValue(ctx context.Context, workspace string, req khclient.CreateKeyValueRequest) error
	UpdateKeyValue(ctx context.Context, workspace, key string, req khclient.UpdateKeyValueRequest) error
	DeleteKeyValue(ctx context.Context, workspace, key string) error
}

type remoteKVStore struct {
	client *khclient.Client
}

func (s *remoteKVStore) ListKeyValues(ctx context.Context, workspace string) ([]khclient.KeyValue, error) {
	return s.client.ListKeyValues(ctx, workspace)
}

func (s *remoteKVStore) GetKeyValue(ctx context.Context, _ string, key string) (khclient.KeyValue, error) {
	return s.client.GetKeyValue(ctx, key)
}

func (s *remoteKVStore) CreateKeyValue(ctx context.Context, workspace string, req khclient.CreateKeyValueRequest) error {
	return s.client.CreateKeyValue(ctx, workspace, req)
}

func (s *remoteKVStore) UpdateKeyValue(ctx context.Context, _ string, key string, req khclient.UpdateKeyValueRequest) error {
	return s.client.UpdateKeyValue(ctx, key, req)
}

func (s *remoteKVStore) DeleteKeyValue(ctx context.Context, _ string, key string) error {
	return s.client.DeleteKeyValue(ctx, key)
}

type fileKVStore struct {
	path string
}

type fileKVData struct {
	Version    int                               `json:"version"`
	Workspaces map[string]map[string]fileKVEntry `json:"workspaces"`
}

type fileKVEntry struct {
	Value       string  `json:"value"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
	Private     bool    `json:"private,omitempty"`
	OneTimeOnly bool    `json:"one_time_only,omitempty"`
	Environment string  `json:"environment,omitempty"`
}

func defaultFileKVData() fileKVData {
	return fileKVData{Version: 1, Workspaces: map[string]map[string]fileKVEntry{}}
}

func (s *fileKVStore) load() (fileKVData, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultFileKVData(), nil
		}
		return fileKVData{}, err
	}
	if len(b) == 0 {
		return defaultFileKVData(), nil
	}
	var data fileKVData
	if err := json.Unmarshal(b, &data); err != nil {
		return fileKVData{}, err
	}
	if data.Workspaces == nil {
		data.Workspaces = map[string]map[string]fileKVEntry{}
	}
	if data.Version == 0 {
		data.Version = 1
	}
	return data, nil
}

func (s *fileKVStore) save(data fileKVData) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func workspaceMap(data *fileKVData, workspace string) map[string]fileKVEntry {
	m, ok := data.Workspaces[workspace]
	if !ok {
		m = map[string]fileKVEntry{}
		data.Workspaces[workspace] = m
	}
	return m
}

func (s *fileKVStore) ListKeyValues(_ context.Context, workspace string) ([]khclient.KeyValue, error) {
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	m := workspaceMap(&data, workspace)
	out := make([]khclient.KeyValue, 0, len(m))
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry := m[key]
		out = append(out, khclient.KeyValue{
			Key:         key,
			Value:       entry.Value,
			RawValue:    []byte(entry.Value),
			ExpiresAt:   entry.ExpiresAt,
			Private:     entry.Private,
			OneTimeOnly: entry.OneTimeOnly,
			Environment: entry.Environment,
		})
	}
	return out, nil
}

func (s *fileKVStore) GetKeyValue(_ context.Context, workspace, key string) (khclient.KeyValue, error) {
	data, err := s.load()
	if err != nil {
		return khclient.KeyValue{}, err
	}
	m := workspaceMap(&data, workspace)
	entry, ok := m[key]
	if !ok {
		return khclient.KeyValue{}, khclient.APIError{StatusCode: 404, Message: "key not found in file backend"}
	}
	if entry.OneTimeOnly {
		delete(m, key)
		if err := s.save(data); err != nil {
			return khclient.KeyValue{}, err
		}
	}
	return khclient.KeyValue{
		Key:         key,
		Value:       entry.Value,
		RawValue:    []byte(entry.Value),
		ExpiresAt:   entry.ExpiresAt,
		Private:     entry.Private,
		OneTimeOnly: entry.OneTimeOnly,
		Environment: entry.Environment,
	}, nil
}

func (s *fileKVStore) CreateKeyValue(_ context.Context, workspace string, req khclient.CreateKeyValueRequest) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	m := workspaceMap(&data, workspace)
	if _, exists := m[req.Key]; exists {
		return khclient.APIError{StatusCode: 422, Message: fmt.Sprintf("key %q already exists in file backend", req.Key)}
	}
	m[req.Key] = fileKVEntry{
		Value:       req.Payload,
		ExpiresAt:   req.ExpiresAt,
		Private:     req.Private,
		OneTimeOnly: req.OneTimeOnly,
	}
	return s.save(data)
}

func (s *fileKVStore) UpdateKeyValue(_ context.Context, workspace, key string, req khclient.UpdateKeyValueRequest) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	m := workspaceMap(&data, workspace)
	entry, exists := m[key]
	if !exists {
		return khclient.APIError{StatusCode: 404, Message: "key not found in file backend"}
	}
	entry.Value = req.Payload
	if req.ExpiresAt != nil {
		entry.ExpiresAt = req.ExpiresAt
	}
	if req.Private != nil {
		entry.Private = *req.Private
	}
	if req.OneTimeOnly != nil {
		entry.OneTimeOnly = *req.OneTimeOnly
	}
	m[key] = entry
	return s.save(data)
}

func (s *fileKVStore) DeleteKeyValue(_ context.Context, workspace, key string) error {
	data, err := s.load()
	if err != nil {
		return err
	}
	m := workspaceMap(&data, workspace)
	if _, exists := m[key]; !exists {
		return khclient.APIError{StatusCode: 404, Message: "key not found in file backend"}
	}
	delete(m, key)
	return s.save(data)
}
