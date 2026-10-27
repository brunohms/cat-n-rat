// Package stats keeps the purchase aggregates of each customer (customer_stats) up to date.
package stats

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// StatusPaid is the only status whose total counts as money spent.
const StatusPaid = "paid"

// Message is a purchase stored by the backend, as published to Kafka (purchase.StoredMessage in the backend).
type Message struct {
	EventID        uuid.UUID   `json:"event_id"`
	OrderID        string      `json:"order_id"`
	CustomerID     string      `json:"customer_id"`
	Status         string      `json:"status"`
	Total          json.Number `json:"total"`
	EventTimestamp time.Time   `json:"event_timestamp"`
}

// Decode parses and validates a message; an error means the message can never be applied.
func Decode(value []byte) (Message, error) {
	var m Message
	if err := json.Unmarshal(value, &m); err != nil {
		return m, fmt.Errorf("invalid JSON: %w", err)
	}

	var errs []error
	if m.EventID == uuid.Nil {
		errs = append(errs, errors.New("event_id is required"))
	}
	if m.CustomerID == "" {
		errs = append(errs, errors.New("customer_id is required"))
	}
	if m.Status == "" {
		errs = append(errs, errors.New("status is required"))
	}
	if _, err := m.Total.Float64(); err != nil {
		errs = append(errs, errors.New("total must be a number"))
	}
	if m.EventTimestamp.IsZero() {
		errs = append(errs, errors.New("event_timestamp is required"))
	}
	return m, errors.Join(errs...)
}

// spent is the amount the purchase adds to total_spent.
func (m Message) spent() string {
	if m.Status == StatusPaid {
		return m.Total.String()
	}
	return "0"
}
