package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	configFile = "config.json"
)

type Category struct {
	Name         string `json:"name"`
	NumberSounds int    `json:"number_sounds"`
}

type Config struct {
	ClientId         string     `json:"client_id"`
	ClientSecret     string     `json:"client_secret"`
	SearchPrefix     string     `json:"search_prefix"`
	MaxDuration      float64    `json:"max_duration"`
	DownloadTimeout  int        `json:"download_timeout_seconds"`
	OutputDir        string     `json:"output_dir"`
	Categories       []Category `json:"categories"`
	SamplesSource    string     `json:"samples_source"`
	GoogleChromePath string     `json:"google_chrome_path"`
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if config.MaxDuration == 0 {
		config.MaxDuration = 5.0
	}

	if config.DownloadTimeout == 0 {
		config.DownloadTimeout = 10
	}

	if config.OutputDir == "" {
		config.OutputDir = "./sounds"
	}

	return &config, nil
}
