// Package config loads the service settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// PostgreSQL connection URL
	DatabaseURL string
	// Base URL of the generator service, used to request events on demand
	GeneratorURL string
	// Timeout for requests to the generator
	GeneratorTimeout time.Duration
	// Maximum number of events per ingestion or fetch request
	MaxBatch int
	// Applies pending migrations when the server starts
	RunMigrations bool
	// HTTP port
	Port string
	// Kafka bootstrap brokers
	KafkaBrokers []string
	// Topic of the messages published for every new purchase
	KafkaTopic string
}

func Load() (Config, error) {
	timeout, err := strconv.ParseFloat(getenv("GENERATOR_TIMEOUT", "30"), 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid GENERATOR_TIMEOUT: %w", err)
	}
	maxBatch, err := strconv.Atoi(getenv("MAX_BATCH", "10000"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid MAX_BATCH: %w", err)
	}
	runMigrations, err := strconv.ParseBool(getenv("RUN_MIGRATIONS", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid RUN_MIGRATIONS: %w", err)
	}

	return Config{
		DatabaseURL:      getenv("DATABASE_URL", "postgres://catnrat:catnrat@localhost:5432/catnrat?sslmode=disable"),
		GeneratorURL:     getenv("GENERATOR_URL", "http://localhost:8000"),
		GeneratorTimeout: time.Duration(timeout * float64(time.Second)),
		MaxBatch:         maxBatch,
		RunMigrations:    runMigrations,
		Port:             getenv("PORT", "8000"),
		KafkaBrokers:     strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:       getenv("KAFKA_TOPIC", "purchases.stored"),
	}, nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
