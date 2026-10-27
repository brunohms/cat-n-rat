// Package config loads the worker settings from environment variables.
package config

import (
	"os"
	"strings"
)

type Config struct {
	// PostgreSQL connection URL (the schema is created by the backend migrations)
	DatabaseURL string
	// Kafka bootstrap brokers
	KafkaBrokers []string
	// Topic with the messages published by the backend for every new purchase
	KafkaTopic string
	// Consumer group; the committed offsets are kept per group
	KafkaGroup string
	// HTTP port of the health check
	Port string
}

func Load() Config {
	return Config{
		DatabaseURL:  getenv("DATABASE_URL", "postgres://catnrat:catnrat@localhost:5432/catnrat?sslmode=disable"),
		KafkaBrokers: strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:   getenv("KAFKA_TOPIC", "purchases.stored"),
		KafkaGroup:   getenv("KAFKA_GROUP", "customer-stats"),
		Port:         getenv("PORT", "8000"),
	}
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
