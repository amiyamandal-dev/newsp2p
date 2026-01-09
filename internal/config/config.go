package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	IPFS      IPFSConfig      `mapstructure:"ipfs"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Search    SearchConfig    `mapstructure:"search"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	CORS      CORSConfig      `mapstructure:"cors"`
	P2P       P2PConfig       `mapstructure:"p2p"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"` // debug, release
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Mode         string `mapstructure:"mode"` // "sqlite" or "distributed"
	Path         string `mapstructure:"path"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// IPFSConfig contains IPFS client configuration
type IPFSConfig struct {
	APIEndpoint string        `mapstructure:"api_endpoint"`
	Timeout     time.Duration `mapstructure:"timeout"`
	PinArticles bool          `mapstructure:"pin_articles"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret          string        `mapstructure:"jwt_secret"`
	JWTExpiry          time.Duration `mapstructure:"jwt_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	BcryptCost         int           `mapstructure:"bcrypt_cost"`
}

// SearchConfig contains search index configuration
type SearchConfig struct {
	IndexPath string `mapstructure:"index_path"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
	Burst             int `mapstructure:"burst"`
}

// CORSConfig contains CORS configuration
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// P2PConfig contains P2P network configuration
type P2PConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	ListenAddrs    []string `mapstructure:"listen_addrs"`
	BootstrapPeers []string `mapstructure:"bootstrap_peers"`
	Rendezvous     string   `mapstructure:"rendezvous"`
}

// Load loads configuration from file and environment variables
// Priority: ENV vars > config.yaml > defaults
func Load() (*Config, error) {
	// Set config file details
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Set defaults
	setDefaults()

	// Bind environment variables
	viper.SetEnvPrefix("NEWS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file (optional - OK if it doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate configuration
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 12345)
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "10s")

	// Database defaults
	viper.SetDefault("database.mode", "sqlite") // sqlite or distributed
	viper.SetDefault("database.path", "./data/news.db")
	viper.SetDefault("database.max_open_conns", 10)
	viper.SetDefault("database.max_idle_conns", 5)

	// IPFS defaults
	viper.SetDefault("ipfs.api_endpoint", "http://localhost:5001")
	viper.SetDefault("ipfs.timeout", "60s")
	viper.SetDefault("ipfs.pin_articles", true)

	// Auth defaults
	viper.SetDefault("auth.jwt_expiry", "24h")
	viper.SetDefault("auth.refresh_token_expiry", "168h") // 7 days
	viper.SetDefault("auth.bcrypt_cost", 12)

	// Search defaults
	viper.SetDefault("search.index_path", "./data/search.bleve")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// Rate limit defaults
	viper.SetDefault("rate_limit.requests_per_minute", 1000)
	viper.SetDefault("rate_limit.burst", 100)

	// CORS defaults
	viper.SetDefault("cors.allowed_origins", []string{"http://localhost:3000"})

	// P2P defaults
	viper.SetDefault("p2p.enabled", true)
	viper.SetDefault("p2p.listen_addrs", []string{
		"/ip4/0.0.0.0/tcp/0",
		"/ip4/0.0.0.0/udp/0/quic-v1",
	})
	viper.SetDefault("p2p.bootstrap_peers", []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	})
	viper.SetDefault("p2p.rendezvous", "newsp2p-network")
}

// validate validates the configuration
func validate(cfg *Config) error {
	// Validate server mode
	if cfg.Server.Mode != "debug" && cfg.Server.Mode != "release" {
		return fmt.Errorf("server.mode must be 'debug' or 'release', got: %s", cfg.Server.Mode)
	}

	// Validate port
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got: %d", cfg.Server.Port)
	}

	// Validate JWT secret
	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret is required")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must be at least 32 characters long")
	}

	// Validate bcrypt cost
	if cfg.Auth.BcryptCost < 10 || cfg.Auth.BcryptCost > 31 {
		return fmt.Errorf("auth.bcrypt_cost must be between 10 and 31, got: %d", cfg.Auth.BcryptCost)
	}

	// Validate logging level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error, got: %s", cfg.Logging.Level)
	}

	// Validate logging format
	if cfg.Logging.Format != "json" && cfg.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be 'json' or 'text', got: %s", cfg.Logging.Format)
	}

	// Validate database mode
	if cfg.Database.Mode != "sqlite" && cfg.Database.Mode != "distributed" {
		return fmt.Errorf("database.mode must be 'sqlite' or 'distributed', got: %s", cfg.Database.Mode)
	}

	// Validate database path
	if cfg.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}

	// Validate IPFS endpoint
	if cfg.IPFS.APIEndpoint == "" {
		return fmt.Errorf("ipfs.api_endpoint is required")
	}

	// Validate search index path
	if cfg.Search.IndexPath == "" {
		return fmt.Errorf("search.index_path is required")
	}

	return nil
}
