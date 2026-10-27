package purchase

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// StoredMessage is published to Kafka for every purchase stored for the first time.
// The worker reads it to update customer_stats; its key is the customer_id, so the messages of a customer
// land on the same partition.
type StoredMessage struct {
	EventID        uuid.UUID   `json:"event_id"`
	OrderID        string      `json:"order_id"`
	CustomerID     string      `json:"customer_id"`
	Status         string      `json:"status"`
	Total          json.Number `json:"total"`
	EventTimestamp time.Time   `json:"event_timestamp"`
}

func newStoredMessage(e Event) StoredMessage {
	return StoredMessage{
		EventID:        e.EventID,
		OrderID:        e.OrderID,
		CustomerID:     e.Customer.CustomerID,
		Status:         e.Status,
		Total:          e.Total,
		EventTimestamp: e.EventTimestamp,
	}
}
