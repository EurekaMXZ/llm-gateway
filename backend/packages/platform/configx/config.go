package configx

import "github.com/caarlos0/env/v11"

// BaseConfig is shared across all services in M1.
type BaseConfig struct {
	Environment string `env:"APP_ENV" envDefault:"local"`
	HTTPAddr    string `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
}

// InfraConfig keeps infrastructure endpoint settings.
// M1 keeps safe local defaults; M2+ may enforce required validation by service.
type InfraConfig struct {
	PostgresDSN  string `env:"POSTGRES_DSN" envDefault:"postgres://gateway:gateway@postgres:5432/llm_gateway?sslmode=disable"`
	RedisAddr    string `env:"REDIS_ADDR" envDefault:"redis:6379"`
	KafkaBrokers string `env:"KAFKA_BROKERS" envDefault:"kafka:9092"`
}

// Config is the shared service config envelope for M1.
type Config struct {
	BaseConfig
	InfraConfig
}

func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadBase() (BaseConfig, error) {
	cfg, err := Load()
	if err != nil {
		return BaseConfig{}, err
	}
	return cfg.BaseConfig, nil
}
