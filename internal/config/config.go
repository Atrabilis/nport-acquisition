package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Site    string        `yaml:"site"`
	Agent   AgentConfig   `yaml:"agent"`
	NPorts  []NPortConfig `yaml:"nports"`
	Storage StorageConfig `yaml:"storage"`
}

type AgentConfig struct {
	Mode                string `yaml:"mode"`
	SubMode             string `yaml:"sub_mode"`
	StoreTimeoutSeconds int    `yaml:"store_timeout_seconds"`
	TestDurationSeconds int    `yaml:"test_duration_seconds"`
	TestOnlyValidCRC    bool   `yaml:"test_only_valid_crc"`
}

type NPortConfig struct {
	Name              string         `yaml:"name"`
	Host              string         `yaml:"host"`
	Port              int            `yaml:"port"`
	DeviceType        string         `yaml:"device_type"`
	IdleGapMS         int            `yaml:"idle_gap_ms"`
	Serial            SerialSettings `yaml:"serial"`
	DialTimeoutMS     int            `yaml:"dial_timeout_ms"`
	ReconnectDelayMS  int            `yaml:"reconnect_delay_ms"`
	ReadBufferBytes   int            `yaml:"read_buffer_bytes"`
	LogFrameHex       bool           `yaml:"log_frame_hex"`
	MaxFrameBytes     int            `yaml:"max_frame_bytes"`
	ConnectionKeepLog bool           `yaml:"connection_keep_log"`
	SkipInvalidCRC    bool           `yaml:"skip_invalid_crc"`
	Slaves            []SlaveConfig  `yaml:"slaves"`
	DetectedSlaves    []uint8        `yaml:"detected_slaves"`
}

type SerialSettings struct {
	Baud     int     `yaml:"baud"`
	DataBits int     `yaml:"data_bits"`
	Parity   string  `yaml:"parity"`
	StopBits float64 `yaml:"stop_bits"`
}

type SlaveConfig struct {
	Address    uint8            `yaml:"address"`
	Name       string           `yaml:"name"`
	DeviceType string           `yaml:"device_type"`
	Registers  []RegisterConfig `yaml:"registers"`
}

type RegisterConfig struct {
	Register      int    `yaml:"register"`
	RegisterName  string `yaml:"register_name"`
	RegisterType  string `yaml:"register_type"`
	RegisterCount int    `yaml:"register_count"`
}

type StorageConfig struct {
	Outputs []StorageOutputConfig `yaml:"outputs"`
}

type StorageOutputConfig struct {
	Name              string                  `yaml:"name"`
	Type              string                  `yaml:"type"`
	Enabled           *bool                   `yaml:"enabled,omitempty"`
	TimescaledbShadow TimescaledbShadowConfig `yaml:"timescaledb_shadow,omitempty"`
}

func (o StorageOutputConfig) IsEnabled() bool {
	return o.Enabled == nil || *o.Enabled
}

type TimescaledbShadowConfig struct {
	HostEnv     string `yaml:"host_env"`
	PortEnv     string `yaml:"port_env"`
	UserEnv     string `yaml:"user_env"`
	PasswordEnv string `yaml:"password_env"`
	DatabaseEnv string `yaml:"database_env"`
	Schema      string `yaml:"schema"`
	Table       string `yaml:"table"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
