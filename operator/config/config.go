package config

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/microsoft/retina/pkg/capture"
	"github.com/microsoft/retina/pkg/config"
	"github.com/spf13/viper"
)

const MinTelemetryInterval time.Duration = 2 * time.Minute

var (
	DefaultTelemetryInterval             = 5 * time.Minute
	ErrorTelemetryIntervalTooSmall       = fmt.Errorf("telemetryInterval smaller than %v is not allowed", MinTelemetryInterval)
	ErrCaptureHostPathBaseDirNotAbsolute = errors.New("captureHostPathBaseDir must be an absolute path")
)

type OperatorConfig struct {
	config.CaptureConfig `mapstructure:",squash"`

	InstallCRDs     bool   `yaml:"installCRDs"`
	EnableTelemetry bool   `yaml:"enableTelemetry"`
	LogLevel        string `yaml:"logLevel"`
	// EnableRetinaEndpoint indicates whether to enable RetinaEndpoint
	EnableRetinaEndpoint bool          `yaml:"enableRetinaEndpoint"`
	RemoteContext        bool          `yaml:"remoteContext"`
	TelemetryInterval    time.Duration `yaml:"telemetryInterval"`
}

func GetConfig(cfgFileName string) (*OperatorConfig, error) {
	viper.SetConfigType("yaml")
	viper.SetConfigFile(cfgFileName)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	viper.AutomaticEnv()

	var cfg OperatorConfig
	viper.SetDefault("EnableRetinaEndpoint", true)
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	// If unset, default telemetry interval to 5 minutes.
	if cfg.TelemetryInterval == 0 {
		log.Printf("telemetryInterval is not set, defaulting to %v", DefaultTelemetryInterval)
		cfg.TelemetryInterval = DefaultTelemetryInterval
	} else if cfg.TelemetryInterval < MinTelemetryInterval {
		return nil, ErrorTelemetryIntervalTooSmall
	}

	// If unset, default the HostPath base directory so that Capture CRs cannot
	// place artifacts arbitrarily on the node filesystem. The CR's HostPath is
	// always joined under this directory.
	if cfg.CaptureHostPathBaseDir == "" {
		log.Printf("captureHostPathBaseDir is not set, defaulting to %s", capture.DefaultHostPathBaseDir)
		cfg.CaptureHostPathBaseDir = capture.DefaultHostPathBaseDir
	}
	cfg.CaptureHostPathBaseDir = filepath.Clean(cfg.CaptureHostPathBaseDir)
	if !filepath.IsAbs(cfg.CaptureHostPathBaseDir) {
		return nil, fmt.Errorf("%w: got %q", ErrCaptureHostPathBaseDirNotAbsolute, cfg.CaptureHostPathBaseDir)
	}

	return &cfg, nil
}
