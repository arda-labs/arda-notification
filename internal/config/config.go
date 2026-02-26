package config

import (
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	TTL      TTLConfig      `mapstructure:"ttl"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Env  string `mapstructure:"env"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type KafkaConfig struct {
	Brokers         []string `mapstructure:"brokers"`
	ConsumerGroupID string   `mapstructure:"consumer_group_id"`
	Topics          []string `mapstructure:"topics"`
}

type KeycloakConfig struct {
	BaseURL string `mapstructure:"base_url"`
	// AdminRealm is the realm used to obtain admin access tokens (usually "master").
	AdminRealm string `mapstructure:"admin_realm"`
	// AdminClientID and AdminClientSecret are credentials for the admin API client.
	AdminClientID     string `mapstructure:"admin_client_id"`
	AdminClientSecret string `mapstructure:"admin_client_secret"`
}

type TTLConfig struct {
	RetentionDays int `mapstructure:"retention_days"` // Default: 30
}

// Load reads configuration from environment variables and config files.
// Environment variables override file values. Prefix: ARDA_NOTIF_
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("server.port", "8090")
	v.SetDefault("server.env", "development")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "arda_notification")
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "password")
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.consumer_group_id", "arda-notification-group")
	v.SetDefault("kafka.topics", []string{"tenant-events", "bpm-events", "crm-events", "iam-events", "notification-commands"})
	v.SetDefault("keycloak.base_url", "http://localhost:8081")
	v.SetDefault("keycloak.admin_realm", "master")
	v.SetDefault("keycloak.admin_client_id", "arda-notification-service")
	v.SetDefault("ttl.retention_days", 30)

	// Environment variables (e.g. DB_HOST -> database.host)
	v.SetEnvPrefix("ARDA_NOTIF")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Also support simple env vars without prefix for Docker Compose convenience
	v.BindEnv("database.host", "DB_HOST")
	v.BindEnv("database.port", "DB_PORT")
	v.BindEnv("database.name", "DB_NAME")
	v.BindEnv("database.user", "DB_USER")
	v.BindEnv("database.password", "DB_PASSWORD")
	v.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	v.BindEnv("keycloak.base_url", "KEYCLOAK_URL")
	v.BindEnv("keycloak.admin_realm", "KEYCLOAK_ADMIN_REALM")
	v.BindEnv("keycloak.admin_client_id", "KEYCLOAK_ADMIN_CLIENT_ID")
	v.BindEnv("keycloak.admin_client_secret", "KEYCLOAK_ADMIN_CLIENT_SECRET")
	v.BindEnv("server.port", "PORT")

	// Try loading config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	_ = v.ReadInConfig() // Not required

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + itoa(d.Port) +
		" dbname=" + d.Name +
		" user=" + d.User +
		" password=" + d.Password +
		" sslmode=disable"
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
