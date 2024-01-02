package cmd

import (
	"bytes"
	"embed"
	"encoding/json"
)

//go:embed settings.json
var content embed.FS

type ClientSettings struct {
	ServerAddress    string `json:"server_address"`
	ListEndpoint     string `json:"list_endpoint"`
	DownloadEndpoint string `json:"download_endpoint"`
	DownloadPrompt   string `json:"download_prompt"`
	ConfigFile       string `json:"config_file"`
}

func ReadClientSettings() (ClientSettings, error) {
	settings, _, _, err := ReadClientSettingsWithRaw()
	return settings, err
}

func ReadClientSettingsWithRaw() (ClientSettings, []byte, int, error) {
	var settings ClientSettings
	raw, err := content.ReadFile("settings.json")
	if err != nil {
		return settings, nil, 0, err
	}
	ind := bytes.LastIndexByte(raw, '}')
	data := raw
	if ind > 0 {
		data = data[:ind+1]
	}
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return settings, nil, 0, err
	}
	return settings, raw, len(data), nil
}

type ServerSettings struct {
	ModsFolder       string `json:"mods_folder"`
	DeleteFile       string `json:"delete_file"`
	ServerPort       uint16 `json:"server_port"`
	ListEndpoint     string `json:"list_endpoint"`
	DownloadEndpoint string `json:"download_endpoint"`
}
