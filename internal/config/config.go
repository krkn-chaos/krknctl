package config

import (
	_ "embed"
	"encoding/json"
)

type Config struct {
	Version      string `json:"version"`
	QuayRegistry string `json:"quayRegistry"`
	QuayApi      string `json:"quayApi"`
}

//go:embed config.json
var File []byte

func LoadConfig() (Config, error) {
	var config Config
	err := json.Unmarshal(File, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}
