// Package purchase contains the purchase event model and its persistence.
package purchase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Event is a purchase event in the format produced by the generator.
// Money values are kept as json.Number so they reach PostgreSQL NUMERIC columns without float rounding.
type Event struct {
	EventID        uuid.UUID   `json:"event_id"`
	EventType      string      `json:"event_type"`
	EventTimestamp time.Time   `json:"event_timestamp"`
	OrderID        string      `json:"order_id"`
	Status         string      `json:"status"`
	Channel        string      `json:"channel"`
	Customer       Customer    `json:"customer"`
	Items          []Item      `json:"items"`
	Payment        Payment     `json:"payment"`
	Currency       string      `json:"currency"`
	Subtotal       json.Number `json:"subtotal"`
	Discount       json.Number `json:"discount"`
	Shipping       json.Number `json:"shipping"`
	Total          json.Number `json:"total"`
}

type Customer struct {
	CustomerID string `json:"customer_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	City       string `json:"city"`
	State      string `json:"state"`
}

type Item struct {
	SKU       string      `json:"sku"`
	Name      string      `json:"name"`
	Category  string      `json:"category"`
	UnitPrice json.Number `json:"unit_price"`
	Quantity  int         `json:"quantity"`
	Subtotal  json.Number `json:"subtotal"`
}

type Payment struct {
	Method       string `json:"method"`
	Installments int    `json:"installments"`
}

// Validate checks that every required field is present and within range.
func (e Event) Validate() error {
	var errs []error
	require := func(field, value string) {
		if value == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}
	requireNumber := func(field string, value json.Number) {
		if _, err := value.Float64(); err != nil {
			errs = append(errs, fmt.Errorf("%s must be a number", field))
		}
	}

	if e.EventID == uuid.Nil {
		errs = append(errs, errors.New("event_id is required"))
	}
	if e.EventType != "purchase" {
		errs = append(errs, errors.New(`event_type must be "purchase"`))
	}
	if e.EventTimestamp.IsZero() {
		errs = append(errs, errors.New("event_timestamp is required"))
	}
	require("order_id", e.OrderID)
	require("status", e.Status)
	require("channel", e.Channel)
	require("customer.customer_id", e.Customer.CustomerID)
	require("customer.name", e.Customer.Name)
	require("customer.email", e.Customer.Email)
	require("customer.city", e.Customer.City)
	require("customer.state", e.Customer.State)
	require("payment.method", e.Payment.Method)
	if e.Payment.Installments < 1 {
		errs = append(errs, errors.New("payment.installments must be at least 1"))
	}
	if len(e.Currency) != 3 {
		errs = append(errs, errors.New("currency must have 3 characters"))
	}
	requireNumber("subtotal", e.Subtotal)
	requireNumber("discount", e.Discount)
	requireNumber("shipping", e.Shipping)
	requireNumber("total", e.Total)

	if len(e.Items) == 0 {
		errs = append(errs, errors.New("items must have at least 1 item"))
	}
	for i, item := range e.Items {
		prefix := fmt.Sprintf("items[%d].", i)
		require(prefix+"sku", item.SKU)
		require(prefix+"name", item.Name)
		require(prefix+"category", item.Category)
		requireNumber(prefix+"unit_price", item.UnitPrice)
		requireNumber(prefix+"subtotal", item.Subtotal)
		if item.Quantity < 1 {
			errs = append(errs, fmt.Errorf("%squantity must be at least 1", prefix))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return errors.New(strings.Join(messages, "; "))
}
