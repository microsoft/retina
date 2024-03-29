package config

import "github.com/microsoft/retina/pkg/config"

type OperatorConfig struct {
	config.CaptureConfig `mapstructure:",squash"`

	InstallCRDs     bool   `yaml:"installCRDs"`
	EnableTelemetry bool   `yaml:"enableTelemetry"`
	LogLevel        string `yaml:"logLevel"`
	// EnableRetinaEndpoint indicates whether to enable RetinaEndpoint
	EnableRetinaEndpoint bool `yaml:"enableRetinaEndpoint"`
	// EnableTrace indicates whether to enable trace. This is a WIP feature.
	EnableTrace   bool `yaml:"enableTrace"`
	RemoteContext bool `yaml:"remoteContext"`
}
