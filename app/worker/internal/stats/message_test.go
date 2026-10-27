package stats

import (
	"strings"
	"testing"
)

const validMessage = `{
	"event_id": "6f1c1a6e-2d2f-4c39-9a8e-3c1d6b2a9f10",
	"order_id": "ORD-1",
	"customer_id": "CUST-1",
	"status": "paid",
	"total": 123.45,
	"event_timestamp": "2026-10-27T12:00:00Z"
}`

func TestDecodeValidMessage(t *testing.T) {
	m, err := Decode([]byte(validMessage))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.CustomerID != "CUST-1" || m.Total.String() != "123.45" {
		t.Fatalf("unexpected message: %+v", m)
	}
}

func TestDecodeRejectsInvalidMessages(t *testing.T) {
	tests := map[string]struct {
		value string
		want  string
	}{
		"not JSON":          {`not json`, "invalid JSON"},
		"missing fields":    {`{}`, "event_id is required"},
		"missing customer":  {strings.Replace(validMessage, `"CUST-1"`, `""`, 1), "customer_id is required"},
		"non-numeric total": {strings.Replace(validMessage, `123.45`, `"abc"`, 1), "invalid JSON"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Decode([]byte(tt.value))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got error %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestSpentOnlyCountsPaidPurchases(t *testing.T) {
	m, err := Decode([]byte(validMessage))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.spent(); got != "123.45" {
		t.Fatalf("paid purchase: got %s, want 123.45", got)
	}
	m.Status = "cancelled"
	if got := m.spent(); got != "0" {
		t.Fatalf("cancelled purchase: got %s, want 0", got)
	}
}
