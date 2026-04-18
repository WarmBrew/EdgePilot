package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	App     AppConfig     `mapstructure:"app"`
	DB      DBConfig      `mapstructure:"db"`
	Redis   RedisConfig   `mapstructure:"redis"`
	JWT     JWTConfig     `mapstructure:"jwt"`
	API     APIConfig     `mapstructure:"api"`
	Encrypt EncryptConfig `mapstructure:"encrypt"`
	Log     LogConfig     `mapstructure:"log"`
	Agent   AgentConfig   `mapstructure:"agent"`
	Storage StorageConfig `mapstructure:"storage"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	SMTP    SMTPConfig    `mapstructure:"smtp"`
	Audit   AuditConfig   `mapstructure:"audit"`
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Name   string `mapstructure:"name"`
	Env    string `mapstructure:"env"`
	Debug  bool   `mapstructure:"debug"`
	Port   int    `mapstructure:"port"`
	Secret string `mapstructure:"secret"`
}

// DBConfig holds PostgreSQL connection settings.
type DBConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// DSN returns the PostgreSQL connection string.
func (c *DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr returns the Redis server address.
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	Expire        string `mapstructure:"expire"`
	RefreshExpire string `mapstructure:"refresh_expire"`
}

// APIConfig holds API-related settings.
type APIConfig struct {
	RateLimit   int      `mapstructure:"rate_limit"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

// EncryptConfig holds encryption settings.
type EncryptConfig struct {
	Key string `mapstructure:"key"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// AgentConfig holds edge agent settings.
type AgentConfig struct {
	TokenSecret       string `mapstructure:"token_secret"`
	HeartbeatInterval int    `mapstructure:"heartbeat_interval"`
	SyncInterval      int    `mapstructure:"sync_interval"`
	MaxBufferSize     int    `mapstructure:"max_buffer_size"`
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Type      string `mapstructure:"type"`
	Path      string `mapstructure:"path"`
	MaxSizeMB int    `mapstructure:"max_size_mb"`
}

// MetricsConfig holds monitoring settings.
type MetricsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

// SMTPConfig holds email settings.
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// AuditConfig holds audit logging settings.
type AuditConfig struct {
	Enabled   bool `mapstructure:"enabled"`
	QueueSize int  `mapstructure:"queue_size"`
	BatchSize int  `mapstructure:"batch_size"`
}

// global config instance.
var globalConfig *Config

// Get returns the global config instance.
func Get() *Config {
	if globalConfig == nil {
		panic("config not initialized, call InitConfig() first")
	}
	return globalConfig
}

// bindings maps viper config keys to environment variable names.
var bindings = map[string]string{
	"app.name":                 "APP_NAME",
	"app.env":                  "APP_ENV",
	"app.debug":                "APP_DEBUG",
	"app.port":                 "APP_PORT",
	"app.secret":               "APP_SECRET",
	"db.host":                  "DB_HOST",
	"db.port":                  "DB_PORT",
	"db.name":                  "DB_NAME",
	"db.user":                  "DB_USER",
	"db.password":              "DB_PASSWORD",
	"db.sslmode":               "DB_SSLMODE",
	"db.max_open_conns":        "DB_MAX_OPEN_CONNS",
	"db.max_idle_conns":        "DB_MAX_IDLE_CONNS",
	"redis.host":               "REDIS_HOST",
	"redis.port":               "REDIS_PORT",
	"redis.password":           "REDIS_PASSWORD",
	"redis.db":                 "REDIS_DB",
	"jwt.secret":               "JWT_SECRET",
	"jwt.expire":               "JWT_EXPIRE",
	"jwt.refresh_expire":       "JWT_REFRESH_EXPIRE",
	"api.rate_limit":           "API_RATE_LIMIT",
	"api.cors_origins":         "API_CORS_ORIGINS",
	"encrypt.key":              "ENCRYPTION_KEY",
	"log.level":                "LOG_LEVEL",
	"log.format":               "LOG_FORMAT",
	"agent.token_secret":       "AGENT_TOKEN_SECRET",
	"agent.heartbeat_interval": "AGENT_HEARTBEAT_INTERVAL",
	"agent.sync_interval":      "AGENT_SYNC_INTERVAL",
	"agent.max_buffer_size":    "AGENT_MAX_BUFFER_SIZE",
	"storage.type":             "STORAGE_TYPE",
	"storage.path":             "STORAGE_PATH",
	"storage.max_size_mb":      "STORAGE_MAX_SIZE_MB",
	"metrics.enabled":          "METRICS_ENABLED",
	"metrics.port":             "METRICS_PORT",
	"smtp.host":                "SMTP_HOST",
	"smtp.port":                "SMTP_PORT",
	"smtp.user":                "SMTP_USER",
	"smtp.password":            "SMTP_PASSWORD",
	"smtp.from":                "SMTP_FROM",
	"audit.enabled":            "AUDIT_ENABLED",
	"audit.queue_size":         "AUDIT_QUEUE_SIZE",
	"audit.batch_size":         "AUDIT_BATCH_SIZE",
}

// InitConfig initializes the configuration from environment variables.
// Should be called after godotenv.Load() if loading from a .env file.
func InitConfig() error {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	v.SetDefault("app.name", "edge-platform")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.debug", true)
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.secret", "")

	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.name", "edge_platform")
	v.SetDefault("db.user", "postgres")
	v.SetDefault("db.password", "")
	v.SetDefault("db.sslmode", "disable")
	v.SetDefault("db.max_open_conns", 25)
	v.SetDefault("db.max_idle_conns", 5)

	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.expire", "24h")
	v.SetDefault("jwt.refresh_expire", "168h")

	v.SetDefault("api.rate_limit", 100)
	v.SetDefault("api.cors_origins", []string{"http://localhost:5173", "http://localhost:3000"})

	v.SetDefault("encrypt.key", "")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("agent.token_secret", "")
	v.SetDefault("agent.heartbeat_interval", 30)
	v.SetDefault("agent.sync_interval", 300)
	v.SetDefault("agent.max_buffer_size", 1000)

	v.SetDefault("storage.type", "local")
	v.SetDefault("storage.path", "./storage")
	v.SetDefault("storage.max_size_mb", 100)

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.port", 9090)

	v.SetDefault("smtp.host", "")
	v.SetDefault("smtp.port", 587)
	v.SetDefault("smtp.user", "")
	v.SetDefault("smtp.password", "")
	v.SetDefault("smtp.from", "")

	v.SetDefault("audit.enabled", true)
	v.SetDefault("audit.queue_size", 10000)
	v.SetDefault("audit.batch_size", 100)

	// Bind each config key to its environment variable
	for key, env := range bindings {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("failed to bind env %s to %s: %w", env, key, err)
		}
	}

	v.AutomaticEnv()

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	corsOrigins := make([]string, 0)
	if len(cfg.API.CORSOrigins) == 1 {
		for _, origin := range strings.Split(cfg.API.CORSOrigins[0], ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				corsOrigins = append(corsOrigins, origin)
			}
		}
		cfg.API.CORSOrigins = corsOrigins
	}

	globalConfig = cfg
	return nil
}
