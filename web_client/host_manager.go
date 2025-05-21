package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type Host struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type HostManager struct {
	File string
}

func NewHostManager() (*HostManager, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(configDir, "ParchClient", "hosts.json")

	// Ensure directory exists
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, err
	}

	return &HostManager{File: path}, nil
}

func (h *HostManager) GetHosts() ([]Host, error) {
	data, err := os.ReadFile(h.File)
	if err != nil {
		return []Host{}, nil
	}
	var hosts []Host
	_ = json.Unmarshal(data, &hosts)
	return hosts, nil
}

func (h *HostManager) VerifyHostKey(hostUUID string, relayBaseURL string) (Host, error) {
	url := fmt.Sprintf(relayBaseURL+"/api/host/%s", hostUUID)

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return Host{}, fmt.Errorf("host not found")
	}
	defer resp.Body.Close()

	var host Host
	err = json.NewDecoder(resp.Body).Decode(&host)
	if err != nil {
		return Host{}, fmt.Errorf("invalid response")
	}

	hosts, _ := h.GetHosts()
	for _, h := range hosts {
		if h.UUID == hostUUID {
			return host, nil
		}
	}

	hosts = append(hosts, Host{UUID: host.UUID, Name: host.Name})
	data, _ := json.MarshalIndent(hosts, "", "  ")
	err = os.WriteFile(h.File, data, 0644)
	if err != nil {
		return Host{}, fmt.Errorf("failed to save host")
	}

	return host, nil
}

func (h *HostManager) RemoveHost(uuid string) error {
	hosts, err := h.GetHosts()
	if err != nil {
		return fmt.Errorf("failed to read hosts: %w", err)
	}

	updated := []Host{}
	found := false
	for _, host := range hosts {
		if host.UUID != uuid {
			updated = append(updated, host)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("host with UUID %s not found", uuid)
	}

	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated hosts: %w", err)
	}

	err = os.WriteFile(h.File, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated host file: %w", err)
	}

	return nil
}
