package config

import (
	"fmt"
	"os"
	"strconv"
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
	AppStore    AppStoreConfig    `yaml:"app_store"`
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

type AppStoreConfig struct {
	BundleID             string   `yaml:"bundle_id"`
	Environment          string   `yaml:"environment"`
	AppAppleID           int64    `yaml:"app_apple_id"`
	EnableOnlineChecks   bool     `yaml:"enable_online_checks"`
	NodePath             string   `yaml:"node_path"`
	VerifierScriptPath   string   `yaml:"verifier_script_path"`
	APIScriptPath        string   `yaml:"api_script_path"`
	RootCertificatePaths []string `yaml:"root_certificate_paths"`
	APIKeyID             string   `yaml:"api_key_id"`
	APIIssuerID          string   `yaml:"api_issuer_id"`
	APIPrivateKeyPath    string   `yaml:"api_private_key_path"`
	APIPrivateKey        string   `yaml:"api_private_key"`
	MonthlyProductID     string   `yaml:"monthly_product_id"`
	LifetimeProductID    string   `yaml:"lifetime_product_id"`
	Timeout              string   `yaml:"timeout"`
	ReconcileEnabled     bool     `yaml:"reconcile_enabled"`
	ReconcileInterval    string   `yaml:"reconcile_interval"`
	ReconcileLookback    string   `yaml:"reconcile_lookback"`
	ReconcileBatchSize   int      `yaml:"reconcile_batch_size"`
	ReconcileMaxPages    int      `yaml:"reconcile_max_pages"`
}

type LoggerConfig struct {
	Level        string `yaml:"level"`
	Path         string `yaml:"path"`
	Rotate       string `yaml:"rotate"`
	MaxAge       string `yaml:"max_age"`
	RotationTime string `yaml:"rotation_time"`
	MaxSizeMB    int    `yaml:"max_size_mb"`
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
		AppStore: AppStoreConfig{
			BundleID:           "hh.spider",
			Environment:        "SANDBOX",
			EnableOnlineChecks: true,
			NodePath:           "node",
			VerifierScriptPath: "apple_iap_verifier/verify_transaction.mjs",
			APIScriptPath:      "apple_iap_verifier/app_store_api.mjs",
			MonthlyProductID:   "hh.spider.vip.monthly",
			LifetimeProductID:  "hh.spider.vip.lifetime",
			Timeout:            "10s",
			ReconcileEnabled:   false,
			ReconcileInterval:  "6h",
			ReconcileLookback:  "720h",
			ReconcileBatchSize: 50,
			ReconcileMaxPages:  10,
		},
		Logger: LoggerConfig{
			Level:        "info",
			Path:         "stdout",
			Rotate:       "%Y%m%d",
			MaxAge:       "168h",
			RotationTime: "24h",
			MaxSizeMB:    100,
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
	c.AppStore.ApplyEnv()
	if c.AppStore.BundleID == "" {
		c.AppStore.BundleID = Default().AppStore.BundleID
	}
	if c.AppStore.Environment == "" {
		c.AppStore.Environment = Default().AppStore.Environment
	}
	if c.AppStore.NodePath == "" {
		c.AppStore.NodePath = Default().AppStore.NodePath
	}
	if c.AppStore.VerifierScriptPath == "" {
		c.AppStore.VerifierScriptPath = Default().AppStore.VerifierScriptPath
	}
	if c.AppStore.APIScriptPath == "" {
		c.AppStore.APIScriptPath = Default().AppStore.APIScriptPath
	}
	if c.AppStore.MonthlyProductID == "" {
		c.AppStore.MonthlyProductID = Default().AppStore.MonthlyProductID
	}
	if c.AppStore.LifetimeProductID == "" {
		c.AppStore.LifetimeProductID = Default().AppStore.LifetimeProductID
	}
	if c.AppStore.Timeout == "" {
		c.AppStore.Timeout = Default().AppStore.Timeout
	}
	if c.AppStore.ReconcileInterval == "" {
		c.AppStore.ReconcileInterval = Default().AppStore.ReconcileInterval
	}
	if c.AppStore.ReconcileLookback == "" {
		c.AppStore.ReconcileLookback = Default().AppStore.ReconcileLookback
	}
	if c.AppStore.ReconcileBatchSize <= 0 {
		c.AppStore.ReconcileBatchSize = Default().AppStore.ReconcileBatchSize
	}
	if c.AppStore.ReconcileMaxPages <= 0 {
		c.AppStore.ReconcileMaxPages = Default().AppStore.ReconcileMaxPages
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
	if c.Logger.MaxSizeMB < 0 {
		c.Logger.MaxSizeMB = 0
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

func (c *AppStoreConfig) ApplyEnv() {
	if value := strings.TrimSpace(os.Getenv("APP_STORE_BUNDLE_ID")); value != "" {
		c.BundleID = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_ENVIRONMENT")); value != "" {
		c.Environment = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_APPLE_ID")); value != "" {
		c.AppAppleID = int64OrDefault(value, c.AppAppleID)
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_APP_APPLE_ID")); value != "" {
		c.AppAppleID = int64OrDefault(value, c.AppAppleID)
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_NODE_PATH")); value != "" {
		c.NodePath = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_VERIFIER_SCRIPT_PATH")); value != "" {
		c.VerifierScriptPath = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_API_SCRIPT_PATH")); value != "" {
		c.APIScriptPath = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_ROOT_CERTIFICATE_PATHS")); value != "" {
		c.RootCertificatePaths = splitCSV(value)
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_API_KEY_ID")); value != "" {
		c.APIKeyID = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_API_ISSUER_ID")); value != "" {
		c.APIIssuerID = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_API_PRIVATE_KEY_PATH")); value != "" {
		c.APIPrivateKeyPath = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_API_PRIVATE_KEY")); value != "" {
		c.APIPrivateKey = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_MONTHLY_PRODUCT_ID")); value != "" {
		c.MonthlyProductID = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_LIFETIME_PRODUCT_ID")); value != "" {
		c.LifetimeProductID = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_TIMEOUT")); value != "" {
		c.Timeout = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_RECONCILE_ENABLED")); value != "" {
		c.ReconcileEnabled = boolOrDefault(value, c.ReconcileEnabled)
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_RECONCILE_INTERVAL")); value != "" {
		c.ReconcileInterval = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_RECONCILE_LOOKBACK")); value != "" {
		c.ReconcileLookback = value
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_RECONCILE_BATCH_SIZE")); value != "" {
		c.ReconcileBatchSize = intOrDefault(value, c.ReconcileBatchSize)
	}
	if value := strings.TrimSpace(os.Getenv("APP_STORE_RECONCILE_MAX_PAGES")); value != "" {
		c.ReconcileMaxPages = intOrDefault(value, c.ReconcileMaxPages)
	}
}

func (c AppStoreConfig) TimeoutDuration() time.Duration {
	return durationOrDefault(c.Timeout, 10*time.Second)
}

func (c AppStoreConfig) ReconcileIntervalDuration() time.Duration {
	return durationOrDefault(c.ReconcileInterval, 6*time.Hour)
}

func (c AppStoreConfig) ReconcileLookbackDuration() time.Duration {
	return durationOrDefault(c.ReconcileLookback, 30*24*time.Hour)
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

func boolOrDefault(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func intOrDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func int64OrDefault(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
