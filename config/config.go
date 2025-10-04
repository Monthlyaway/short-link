package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	MySQL       MySQLConfig       `yaml:"mysql"`
	Redis       RedisConfig       `yaml:"redis"`
	BloomFilter BloomFilterConfig `yaml:"bloom_filter"`
	Snowflake   SnowflakeConfig   `yaml:"snowflake"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

// MySQLConfig represents MySQL configuration
type MySQLConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// BloomFilterConfig represents Bloom filter configuration
type BloomFilterConfig struct {
	Capacity          uint    `yaml:"capacity"`
	FalsePositiveRate float64 `yaml:"false_positive_rate"`
}

// SnowflakeConfig represents Snowflake ID generator configuration
type SnowflakeConfig struct {
	DatacenterID int64 `yaml:"datacenter_id"`
	WorkerID     int64 `yaml:"worker_id"`
}

// DSN returns MySQL data source name
func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.Username, m.Password, m.Host, m.Port, m.Database)
}

// Addr returns Redis address
func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

var globalConfig *Config

// Load loads configuration from file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if present
	if host := os.Getenv("MYSQL_HOST"); host != "" {
		cfg.MySQL.Host = host
	}
	if host := os.Getenv("REDIS_HOST"); host != "" {
		cfg.Redis.Host = host
	}

	globalConfig = &cfg
	return &cfg, nil
}

// Get returns the global configuration
func Get() *Config {
	return globalConfig
}
