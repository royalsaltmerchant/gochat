package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type Host struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type HostManager struct {
	File string
}

func NewHostManager(file string) *HostManager {
	return &HostManager{File: file}
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

func (h *HostManager) VerifyHostKey(hostUUID string) (string, error) {
	url := fmt.Sprintf("http://localhost:8000/api/host/%s", hostUUID)

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return "", fmt.Errorf("host not found")
	}
	defer resp.Body.Close()

	var response struct {
		Name string `json:"name"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return "", fmt.Errorf("invalid response")
	}

	hosts, _ := h.GetHosts()
	for _, h := range hosts {
		if h.UUID == hostUUID {
			return response.Name, nil
		}
	}

	hosts = append(hosts, Host{UUID: hostUUID, Name: response.Name})
	data, _ := json.MarshalIndent(hosts, "", "  ")
	err = os.WriteFile(h.File, data, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save host")
	}

	return response.Name, nil
}
