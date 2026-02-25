// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package config

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Level defines the level of monitor aggregation.
type Level int

// TCXMode controls whether TCX (TC eXpress) attachment is used for packetparser.
type TCXMode string

const (
	// TCXModeAuto detects kernel support and uses TCX if available, falling back to TC.
	TCXModeAuto TCXMode = "auto"
	// TCXModeAlways requires TCX and fails if not supported.
	TCXModeAlways TCXMode = "always"
	// TCXModeOff disables TCX and always uses traditional TC.
	TCXModeOff TCXMode = "off"
)

const MinTelemetryInterval time.Duration = 2 * time.Minute

const (
	Low Level = iota
	High
)

type PacketParserRingBufferMode string

const (
	PacketParserRingBufferDisabled PacketParserRingBufferMode = "disabled"
	PacketParserRingBufferEnabled  PacketParserRingBufferMode = "enabled"
	PacketParserRingBufferAuto     PacketParserRingBufferMode = "auto"
)

var (
	ErrPacketParserRingBufferAutoNotSupported = errors.New("packetParserRingBuffer mode auto is not supported yet")
	ErrPacketParserRingBufferInvalid          = errors.New("packetParserRingBuffer must be set to enabled or disabled")
	ErrPacketParserRingBufferInvalidBool      = errors.New(
		"packetParserRingBuffer must be enabled or disabled, got boolean",
	)
)

var (
	ErrorTelemetryIntervalTooSmall = fmt.Errorf(
		"telemetryInterval smaller than %v is not allowed",
		MinTelemetryInterval,
	)
	DefaultTelemetryInterval              = 15 * time.Minute
	DefaultSamplingRate            uint32 = 1
)

func (l *Level) UnmarshalText(text []byte) error {
	s := strings.ToLower(string(text))
	switch s {
	case "low":
		*l = Low
	case "high":
		*l = High
	default:
		// Default to Low if the text is not recognized.
		*l = Low
	}
	return nil
}

func (l *Level) String() string {
	switch *l {
	case Low:
		return "low"
	case High:
		return "high"
	default:
		return ""
	}
}

func (m *PacketParserRingBufferMode) UnmarshalText(text []byte) error {
	s := strings.ToLower(strings.TrimSpace(string(text)))
	switch s {
	case string(PacketParserRingBufferEnabled):
		*m = PacketParserRingBufferEnabled
		return nil
	case string(PacketParserRingBufferDisabled):
		*m = PacketParserRingBufferDisabled
		return nil
	case string(PacketParserRingBufferAuto):
		return ErrPacketParserRingBufferAutoNotSupported
	case "":
		return ErrPacketParserRingBufferInvalid
	default:
		return fmt.Errorf("invalid packetParserRingBuffer %q: %w", s, ErrPacketParserRingBufferInvalid)
	}
}

func (m *PacketParserRingBufferMode) IsEnabled() bool {
	return *m == PacketParserRingBufferEnabled
}

type Server struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Config struct {
	APIServer       Server        `yaml:"apiServer"`
	LogLevel        string        `yaml:"logLevel"`
	EnabledPlugin   []string      `yaml:"enabledPlugin"`
	MetricsInterval time.Duration `yaml:"metricsInterval"`
	// Deprecated: Use only MetricsInterval instead in the go code.
	MetricsIntervalDuration    time.Duration              `yaml:"metricsIntervalDuration"`
	EnableTelemetry            bool                       `yaml:"enableTelemetry"`
	EnableRetinaEndpoint       bool                       `yaml:"enableRetinaEndpoint"`
	EnablePodLevel             bool                       `yaml:"enablePodLevel"`
	EnableConntrackMetrics     bool                       `yaml:"enableConntrackMetrics"`
	RemoteContext              bool                       `yaml:"remoteContext"`
	EnableAnnotations          bool                       `yaml:"enableAnnotations"`
	BypassLookupIPOfInterest   bool                       `yaml:"bypassLookupIPOfInterest"`
	DataAggregationLevel       Level                      `yaml:"dataAggregationLevel"`
	MonitorSockPath            string                     `yaml:"monitorSockPath"`
	TelemetryInterval          time.Duration              `yaml:"telemetryInterval"`
	DataSamplingRate           uint32                     `yaml:"dataSamplingRate"`
	PacketParserRingBuffer     PacketParserRingBufferMode `yaml:"packetParserRingBuffer"`
	PacketParserRingBufferSize uint32                     `yaml:"packetParserRingBufferSize"`
	EnableTCX                  TCXMode                    `yaml:"enableTCX"`
}

func GetConfig(cfgFilename string) (*Config, error) {
	if cfgFilename != "" {
		viper.SetConfigFile(cfgFilename)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/retina/config")
	}

	viper.SetEnvPrefix("retina")
	viper.AutomaticEnv()
	// NOTE(mainred): RetinaEndpoint is currently the only supported solution to cache Pod, and before an alternative is implemented,
	// we make EnableRetinaEndpoint true and cannot be configurable.
	viper.SetDefault("EnableRetinaEndpoint", true)

	err := viper.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %s", err)
	}
	var config Config
	decoderConfigOption := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(), // default hook.
		mapstructure.StringToSliceHookFunc(","),     // default hook.
		decodeLevelHook,
		decodePacketParserRingBufferModeHook,
	))

	err = viper.Unmarshal(&config, decoderConfigOption)
	if err != nil {
		return nil, fmt.Errorf("fatal error config file: %s", err)
	}

	if config.MetricsIntervalDuration != 0 {
		config.MetricsInterval = config.MetricsIntervalDuration
	} else if config.MetricsInterval != 0 {
		config.MetricsInterval *= time.Second
		log.Print("metricsInterval is deprecated, please use metricsIntervalDuration instead")
	}

	// If unset, default telemetry interval to 15 minutes.
	if config.TelemetryInterval == 0 {
		log.Printf("telemetryInterval is not set, defaulting to %v", DefaultTelemetryInterval)
		config.TelemetryInterval = DefaultTelemetryInterval
	} else if config.TelemetryInterval < MinTelemetryInterval {
		return nil, ErrorTelemetryIntervalTooSmall
	}

	// If unset, default sampling rate to 1
	if config.DataSamplingRate == 0 {
		log.Printf("dataSamplingRate is not set, defaulting to %v", DefaultSamplingRate)
		config.DataSamplingRate = DefaultSamplingRate
	}

	// Default EnableTCX to "auto" if unset.
	if config.EnableTCX == "" {
		config.EnableTCX = TCXModeAuto
	}

	switch config.PacketParserRingBuffer { //nolint:exhaustive // we only care about Auto and empty (default) here
	case "":
		config.PacketParserRingBuffer = PacketParserRingBufferDisabled
	case PacketParserRingBufferAuto:
		return nil, ErrPacketParserRingBufferAutoNotSupported
	}

	return &config, nil
}

func decodeLevelHook(field, target reflect.Type, data interface{}) (interface{}, error) {
	// Check if the field we are decoding is a string.
	if field.Kind() != reflect.String {
		return data, nil
	}
	// Check if the type we are decoding to is a Level.
	if target != reflect.TypeOf(Level(0)) {
		return data, nil
	}
	var level Level
	err := level.UnmarshalText([]byte(data.(string)))
	if err != nil {
		return nil, err
	}
	return level, nil
}

func decodePacketParserRingBufferModeHook(field, target reflect.Type, data interface{}) (interface{}, error) {
	if target != reflect.TypeOf(PacketParserRingBufferMode("")) {
		return data, nil
	}

	switch field.Kind() { //nolint:exhaustive // we only care about String and Bool
	case reflect.String:
		var mode PacketParserRingBufferMode
		if err := mode.UnmarshalText([]byte(data.(string))); err != nil {
			return nil, err
		}
		return mode, nil
	case reflect.Bool:
		return nil, ErrPacketParserRingBufferInvalidBool
	default:
		return data, nil
	}
}
