package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	defaultConfigPath = "config.yaml"
	envConfigPath     = "SPIDER_SERVER_CONFIG"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	MySQL       MySQLConfig       `yaml:"mysql"`
	Session     SessionConfig     `yaml:"session"`
	Auth        AuthConfig        `yaml:"auth"`
	Sign        SignConfig        `yaml:"sign"`
	AppleSignIn AppleSignInConfig `yaml:"apple_sign_in"`
	Logger      LoggerConfig      `yaml:"logger"`
	Client      ClientConfig      `yaml:"client"`
}

type ServerConfig struct {
	GatewayAddr       string `yaml:"gateway_addr"`
	GRPCAddr          string `yaml:"grpc_addr"`
	GRPCTarget        string `yaml:"grpc_target"`
	EndpointHost      string `yaml:"endpoint_host"`
	ReadHeaderTimeout string `yaml:"read_header_timeout"`
}

type MySQLConfig struct {
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Database        string `yaml:"database"`
	Charset         string `yaml:"charset"`
	ParseTime       bool   `yaml:"parse_time"`
	Loc             string `yaml:"loc"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime string `yaml:"conn_max_idle_time"`
	LogLevel        string `yaml:"log_level"`
}

type SessionConfig struct {
	SignSecret string `yaml:"sign_secret"`
	DefaultTTL string `yaml:"default_ttl"`
}

type AuthConfig struct {
	PublicGRPCMethodPrefixes []string `yaml:"public_grpc_method_prefixes"`
}

type SignConfig struct {
	Enabled               bool   `yaml:"enabled"`
	ReplayNonceTTL        string `yaml:"replay_nonce_ttl"`
	ReplayNonceCleanup    string `yaml:"replay_nonce_cleanup"`
	LogMetadataPrefixOnly bool   `yaml:"log_metadata_prefix_only"`
}

type AppleSignInConfig struct {
	TeamID          string `yaml:"team_id"`
	KeyID           string `yaml:"key_id"`
	ClientID        string `yaml:"client_id"`
	PrivateKeyPath  string `yaml:"private_key_path"`
	PrivateKey      string `yaml:"private_key"`
	ClientSecretTTL string `yaml:"client_secret_ttl"`
}

type LoggerConfig struct {
	Level        string `yaml:"level"`
	Path         string `yaml:"path"`
	Rotate       string `yaml:"rotate"`
	MaxAge       string `yaml:"max_age"`
	RotationTime string `yaml:"rotation_time"`
}

type ClientConfig struct {
	GatewayBaseURL string `yaml:"gateway_base_url"`
	Timeout        string `yaml:"timeout"`
	SignSalt       string `yaml:"sign_salt"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			GatewayAddr:       ":19080",
			GRPCAddr:          ":18000",
			GRPCTarget:        "localhost:18000",
			EndpointHost:      "127.0.0.1",
			ReadHeaderTimeout: "5s",
		},
		MySQL: MySQLConfig{
			User:            "root",
			Password:        "root",
			Host:            "localhost",
			Port:            3306,
			Database:        "spider",
			Charset:         "utf8mb4",
			ParseTime:       true,
			Loc:             "Local",
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: "1h",
			ConnMaxIdleTime: "10m",
			LogLevel:        "warn",
		},
		Session: SessionConfig{
			SignSecret: "spider-sign-session-secret",
			DefaultTTL: "8760h",
		},
		Auth: AuthConfig{
			PublicGRPCMethodPrefixes: []string{"/uc."},
		},
		Sign: SignConfig{
			Enabled:               true,
			ReplayNonceTTL:        "60s",
			ReplayNonceCleanup:    "5s",
			LogMetadataPrefixOnly: true,
		},
		AppleSignIn: AppleSignInConfig{
			TeamID:          "XLVU7GGT6N",
			KeyID:           "5LFYA472TZ",
			ClientID:        "hh.spider",
			ClientSecretTTL: "24h",
		},
		Logger: LoggerConfig{
			Level:        "info",
			Path:         "stdout",
			Rotate:       "%Y%m%d%H",
			MaxAge:       "24h",
			RotationTime: "1h",
		},
		Client: ClientConfig{
			GatewayBaseURL: "http://127.0.0.1:19080",
			Timeout:        "10s",
			SignSalt:       "",
		},
	}
}

func LoadDefault() (Config, error) {
	return Load("")
}

func Load(path string) (Config, error) {
	cfg := Default()
	path = strings.TrimSpace(path)
	explicit := path != ""
	if path == "" {
		path = strings.TrimSpace(os.Getenv(envConfigPath))
		explicit = path != ""
	}
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			cfg.Normalize()
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s failed: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s failed: %w", path, err)
	}

	cfg.Normalize()
	return cfg, nil
}

func (c *Config) Normalize() {
	if c.Server.GatewayAddr == "" {
		c.Server.GatewayAddr = Default().Server.GatewayAddr
	}
	if c.Server.GRPCAddr == "" {
		c.Server.GRPCAddr = Default().Server.GRPCAddr
	}
	if c.Server.GRPCTarget == "" {
		c.Server.GRPCTarget = Default().Server.GRPCTarget
	}
	if c.Server.EndpointHost == "" {
		c.Server.EndpointHost = Default().Server.EndpointHost
	}
	if c.Server.ReadHeaderTimeout == "" {
		c.Server.ReadHeaderTimeout = Default().Server.ReadHeaderTimeout
	}
	if c.Session.SignSecret == "" {
		c.Session.SignSecret = Default().Session.SignSecret
	}
	if c.Session.DefaultTTL == "" {
		c.Session.DefaultTTL = Default().Session.DefaultTTL
	}
	if len(c.Auth.PublicGRPCMethodPrefixes) == 0 {
		c.Auth.PublicGRPCMethodPrefixes = Default().Auth.PublicGRPCMethodPrefixes
	}
	if c.Sign.ReplayNonceTTL == "" {
		c.Sign.ReplayNonceTTL = Default().Sign.ReplayNonceTTL
	}
	if c.Sign.ReplayNonceCleanup == "" {
		c.Sign.ReplayNonceCleanup = Default().Sign.ReplayNonceCleanup
	}
	c.AppleSignIn.ApplyEnv()
	if c.AppleSignIn.TeamID == "" {
		c.AppleSignIn.TeamID = Default().AppleSignIn.TeamID
	}
	if c.AppleSignIn.KeyID == "" {
		c.AppleSignIn.KeyID = Default().AppleSignIn.KeyID
	}
	if c.AppleSignIn.ClientID == "" {
		c.AppleSignIn.ClientID = Default().AppleSignIn.ClientID
	}
	if c.AppleSignIn.ClientSecretTTL == "" {
		c.AppleSignIn.ClientSecretTTL = Default().AppleSignIn.ClientSecretTTL
	}
	if c.Logger.Level == "" {
		c.Logger.Level = Default().Logger.Level
	}
	if c.Logger.Path == "" {
		c.Logger.Path = Default().Logger.Path
	}
	if c.Logger.Rotate == "" {
		c.Logger.Rotate = Default().Logger.Rotate
	}
	if c.Logger.MaxAge == "" {
		c.Logger.MaxAge = Default().Logger.MaxAge
	}
	if c.Logger.RotationTime == "" {
		c.Logger.RotationTime = Default().Logger.RotationTime
	}
	if c.Client.GatewayBaseURL == "" {
		c.Client.GatewayBaseURL = Default().Client.GatewayBaseURL
	}
	if c.Client.Timeout == "" {
		c.Client.Timeout = Default().Client.Timeout
	}
}

func (c ServerConfig) ReadHeaderTimeoutDuration() time.Duration {
	return durationOrDefault(c.ReadHeaderTimeout, 5*time.Second)
}

func (c MySQLConfig) ConnMaxLifetimeDuration() time.Duration {
	return durationOrDefault(c.ConnMaxLifetime, time.Hour)
}

func (c MySQLConfig) ConnMaxIdleTimeDuration() time.Duration {
	return durationOrDefault(c.ConnMaxIdleTime, 10*time.Minute)
}

func (c SessionConfig) DefaultTTLDuration() time.Duration {
	return durationOrDefault(c.DefaultTTL, 365*24*time.Hour)
}

func (c SignConfig) ReplayNonceTTLDuration() time.Duration {
	return durationOrDefault(c.ReplayNonceTTL, time.Minute)
}

func (c SignConfig) ReplayNonceCleanupDuration() time.Duration {
	return durationOrDefault(c.ReplayNonceCleanup, 5*time.Second)
}

func (c *AppleSignInConfig) ApplyEnv() {
	if value := strings.TrimSpace(os.Getenv("APPLE_TEAM_ID")); value != "" {
		c.TeamID = value
	}
	if value := strings.TrimSpace(os.Getenv("APPLE_KEY_ID")); value != "" {
		c.KeyID = value
	}
	if value := strings.TrimSpace(os.Getenv("APPLE_CLIENT_ID")); value != "" {
		c.ClientID = value
	}
	if value := strings.TrimSpace(os.Getenv("APPLE_PRIVATE_KEY_PATH")); value != "" {
		c.PrivateKeyPath = value
	}
	if value := strings.TrimSpace(os.Getenv("APPLE_PRIVATE_KEY")); value != "" {
		c.PrivateKey = value
	}
	if value := strings.TrimSpace(os.Getenv("APPLE_CLIENT_SECRET_TTL")); value != "" {
		c.ClientSecretTTL = value
	}
}

func (c AppleSignInConfig) ClientSecretTTLDuration() time.Duration {
	return durationOrDefault(c.ClientSecretTTL, 24*time.Hour)
}

func (c LoggerConfig) MaxAgeDuration() time.Duration {
	return durationOrDefault(c.MaxAge, 24*time.Hour)
}

func (c LoggerConfig) RotationTimeDuration() time.Duration {
	return durationOrDefault(c.RotationTime, time.Hour)
}

func (c ClientConfig) TimeoutDuration() time.Duration {
	return durationOrDefault(c.Timeout, 10*time.Second)
}

func durationOrDefault(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}
