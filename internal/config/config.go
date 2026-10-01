package config

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Environment  string
	Port         int
	OTLPEndpoint string
	DatabaseURL  string
	JWTSecret    string
}

var envFlag = flag.String("env", "development", "environment to use")
var portFlag = flag.Int("port", 8080, "port to listen on")
var otlpEndpointFlag = flag.String("otlp-endpoint", "localhost:4317", "otlp endpoint")
var databaseUrlFlag = flag.String("database-url", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable&timezone=utc", "database connection string")
var jwtSecretFlag = flag.String("jwt-secret", "", "JWT secret")

func Load() *Config {
	flag.Parse()

	cfg := &Config{
		Environment:  *envFlag,
		Port:         *portFlag,
		OTLPEndpoint: *otlpEndpointFlag,
		DatabaseURL:  *databaseUrlFlag,
		JWTSecret:    *jwtSecretFlag,
	}

	assignStringFromEnv("DTTMR_ENVIRONMENT", &cfg.Environment)
	assignIntFromEnv("DTTMR_PORT", &cfg.Port)
	assignStringFromEnv("DTTMR_OTLP_ENDPOINT", &cfg.OTLPEndpoint)
	assignStringFromEnv("DTTMR_DATABASE_URL", &cfg.DatabaseURL)
	assignStringFromEnv("DTTMR_JWT_SECRET", &cfg.JWTSecret)

	return cfg
}

func assignStringFromEnv(key string, target *string) {
	if val, exists := os.LookupEnv(key); exists {
		*target = val
	}
}

func assignIntFromEnv(key string, target *int) {
	if val, exists := os.LookupEnv(key); exists {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			slog.Error("failed to parse environment variable",
				slog.String("key", key),
				slog.String("value", val),
				slog.Any("error", err))
		} else {
			*target = parsed
		}
	}
}
