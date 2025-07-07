package handler

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// Config holds application configuration
type Config struct {
	WebhookSecretToken string `yaml:"webhook_secret_token"`
	ViewerPassword     string `yaml:"viewer_password"`
	Path               string
}

var ConfigInstance *Config

func NewConfig(configPath string) (*Config, error) {
	config := &Config{}

	file, err := os.Open(configPath)
	if err != nil {
		wd, _ := os.Getwd()
		return nil, fmt.Errorf("could not load config file, looking for %v in %v: %w", configPath, wd, err)
	}
	defer file.Close()

	d := yaml.NewDecoder(file)

	if err := d.Decode(&config); err != nil {
		return nil, fmt.Errorf("could not decode config file: %w", err)
	}
	config.Path = configPath[:len(configPath)-len("config.yml")]
	return config, nil
}
