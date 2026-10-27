package purchase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	SourcePush = "push"
	SourcePull = "pull"
)

var ErrNotFound = errors.New("purchase not found")

type IngestResult struct {
	// Number of events in the request
	Received int `json:"received"`
	// Number of new events stored
	Inserted int `json:"inserted"`
	// Number of events already stored, ignored
	Duplicates int `json:"duplicates"`
}

// Purchase is a stored purchase as returned by the API.
type Purchase struct {
	EventID        uuid.UUID   `json:"event_id"`
	OrderID        string      `json:"order_id"`
	EventTimestamp time.Time   `json:"event_timestamp"`
	Status         string      `json:"status"`
	Channel        string      `json:"channel"`
	PaymentMethod  string      `json:"payment_method"`
	Installments   int         `json:"installments"`
	Currency       string      `json:"currency"`
	Subtotal       json.Number `json:"subtotal"`
	Discount       json.Number `json:"discount"`
	Shipping       json.Number `json:"shipping"`
	Total          json.Number `json:"total"`
	Source         string      `json:"source"`
	ReceivedAt     time.Time   `json:"received_at"`
	Customer       Customer    `json:"customer"`
	Items          []Item      `json:"items"`
}

// ListFilter selects purchases; empty fields are ignored.
type ListFilter struct {
	Limit         int
	Offset        int
	Status        string
	Channel       string
	PaymentMethod string
	Source        string
	CustomerID    string
	// Case-insensitive match on order_id, customer name or customer email
	Search string
	// Event time range: From is inclusive, To is exclusive
	From *time.Time
	To   *time.Time
	// Total range, inclusive, as decimal strings
	MinTotal string
	MaxTotal string
	// One of PurchaseSortFields; empty means event_timestamp
	Sort string
	Desc bool
}

// PurchaseSortFields maps the accepted sort keys to their SQL expressions.
var PurchaseSortFields = map[string]string{
	"event_timestamp": "p.event_timestamp",
	"received_at":     "p.received_at",
	"total":           "p.total",
	"order_id":        "p.order_id",
	"customer":        "c.name",
	"status":          "p.status",
}

// Page is one page of a filtered list, with the total number of matching rows.
type Page[T any] struct {
	Data   []T `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Store struct {
	pool *pgxpool.Pool
	// Kafka topic of the StoredMessage of each new purchase
	topic string
}

func NewStore(pool *pgxpool.Pool, topic string) *Store {
	return &Store{pool: pool, topic: topic}
}

// Save stores purchase events idempotently: events whose event_id already exists are skipped.
// Each new purchase also gets a StoredMessage in the outbox, in the same transaction.
func (s *Store) Save(ctx context.Context, events []Event, source string) (IngestResult, error) {
	result := IngestResult{Received: len(events)}
	if len(events) == 0 {
		return result, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := upsertCustomers(ctx, tx, events); err != nil {
		return result, err
	}
	inserted, err := insertPurchases(ctx, tx, events, source)
	if err != nil {
		return result, err
	}
	if err := insertItems(ctx, tx, events, inserted); err != nil {
		return result, err
	}
	if err := s.insertOutbox(ctx, tx, events, inserted); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit: %w", err)
	}

	result.Inserted = len(inserted)
	result.Duplicates = result.Received - result.Inserted
	return result, nil
}

func upsertCustomers(ctx context.Context, tx pgx.Tx, events []Event) error {
	// Latest data wins for customers that appear more than once
	latest := map[string]Customer{}
	for _, e := range events {
		latest[e.Customer.CustomerID] = e.Customer
	}
	// Sorted so concurrent batches lock rows in the same order and do not deadlock
	ids := make([]string, 0, len(latest))
	for id := range latest {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var names, emails, cities, states []string
	for _, id := range ids {
		c := latest[id]
		names = append(names, c.Name)
		emails = append(emails, c.Email)
		cities = append(cities, c.City)
		states = append(states, c.State)
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO customers (customer_id, name, email, city, state)
		SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[])
		ON CONFLICT (customer_id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			city = EXCLUDED.city,
			state = EXCLUDED.state,
			updated_at = now()`,
		ids, names, emails, cities, states)
	if err != nil {
		return fmt.Errorf("upsert customers: %w", err)
	}
	return nil
}

// insertPurchases returns the events that were actually inserted, keyed by event_id.
func insertPurchases(ctx context.Context, tx pgx.Tx, events []Event, source string) (map[uuid.UUID]Event, error) {
	unique := map[uuid.UUID]Event{}
	var (
		ids, orderIDs, statuses, channels, customerIDs, methods, currencies []string
		subtotals, discounts, shippings, totals                             []string
		timestamps                                                          []time.Time
		installments                                                        []int32
		payloads                                                            [][]byte
	)
	for _, e := range events {
		if _, seen := unique[e.EventID]; seen {
			continue
		}
		unique[e.EventID] = e
		payload, err := json.Marshal(e)
		if err != nil {
			return nil, fmt.Errorf("encode payload: %w", err)
		}
		ids = append(ids, e.EventID.String())
		orderIDs = append(orderIDs, e.OrderID)
		timestamps = append(timestamps, e.EventTimestamp)
		statuses = append(statuses, e.Status)
		channels = append(channels, e.Channel)
		customerIDs = append(customerIDs, e.Customer.CustomerID)
		methods = append(methods, e.Payment.Method)
		installments = append(installments, int32(e.Payment.Installments))
		currencies = append(currencies, e.Currency)
		subtotals = append(subtotals, e.Subtotal.String())
		discounts = append(discounts, e.Discount.String())
		shippings = append(shippings, e.Shipping.String())
		totals = append(totals, e.Total.String())
		payloads = append(payloads, payload)
	}

	rows, err := tx.Query(ctx, `
		INSERT INTO purchases (
			event_id, order_id, event_timestamp, status, channel, customer_id, payment_method,
			installments, currency, subtotal, discount, shipping, total, source, payload
		)
		SELECT
			t.event_id::uuid, t.order_id, t.event_timestamp, t.status, t.channel, t.customer_id, t.payment_method,
			t.installments, t.currency, t.subtotal::numeric, t.discount::numeric, t.shipping::numeric,
			t.total::numeric, $14, t.payload::jsonb
		FROM unnest(
			$1::text[], $2::text[], $3::timestamptz[], $4::text[], $5::text[], $6::text[], $7::text[],
			$8::int[], $9::text[], $10::text[], $11::text[], $12::text[], $13::text[], $15::text[]
		) AS t(
			event_id, order_id, event_timestamp, status, channel, customer_id, payment_method,
			installments, currency, subtotal, discount, shipping, total, payload
		)
		ON CONFLICT (event_id) DO NOTHING
		RETURNING event_id`,
		ids, orderIDs, timestamps, statuses, channels, customerIDs, methods,
		installments, currencies, subtotals, discounts, shippings, totals, source, payloadsAsText(payloads))
	if err != nil {
		return nil, fmt.Errorf("insert purchases: %w", err)
	}
	insertedIDs, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("insert purchases: %w", err)
	}

	inserted := make(map[uuid.UUID]Event, len(insertedIDs))
	for _, id := range insertedIDs {
		inserted[id] = unique[id]
	}
	return inserted, nil
}

func payloadsAsText(payloads [][]byte) []string {
	text := make([]string, len(payloads))
	for i, p := range payloads {
		text[i] = string(p)
	}
	return text
}

func insertItems(ctx context.Context, tx pgx.Tx, events []Event, inserted map[uuid.UUID]Event) error {
	var (
		eventIDs, skus, names, categories, unitPrices, subtotals []string
		quantities                                               []int32
	)
	// Iterates over the request order so items keep a stable id order
	done := map[uuid.UUID]bool{}
	for _, e := range events {
		if _, ok := inserted[e.EventID]; !ok || done[e.EventID] {
			continue
		}
		done[e.EventID] = true
		for _, item := range e.Items {
			eventIDs = append(eventIDs, e.EventID.String())
			skus = append(skus, item.SKU)
			names = append(names, item.Name)
			categories = append(categories, item.Category)
			unitPrices = append(unitPrices, item.UnitPrice.String())
			quantities = append(quantities, int32(item.Quantity))
			subtotals = append(subtotals, item.Subtotal.String())
		}
	}
	if len(eventIDs) == 0 {
		return nil
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO purchase_items (event_id, sku, name, category, unit_price, quantity, subtotal)
		SELECT t.event_id::uuid, t.sku, t.name, t.category, t.unit_price::numeric, t.quantity, t.subtotal::numeric
		FROM unnest($1::text[], $2::text[], $3::text[], $4::text[], $5::text[], $6::int[], $7::text[])
			WITH ORDINALITY AS t(event_id, sku, name, category, unit_price, quantity, subtotal, position)
		ORDER BY t.position`,
		eventIDs, skus, names, categories, unitPrices, quantities, subtotals)
	if err != nil {
		return fmt.Errorf("insert purchase items: %w", err)
	}
	return nil
}

// insertOutbox queues a StoredMessage for each inserted event, in the request order.
func (s *Store) insertOutbox(ctx context.Context, tx pgx.Tx, events []Event, inserted map[uuid.UUID]Event) error {
	// The trace context of the current request, so the publish and the worker spans join its trace
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	headers, err := json.Marshal(carrier)
	if err != nil {
		return fmt.Errorf("encode outbox headers: %w", err)
	}

	var keys, payloads []string
	done := map[uuid.UUID]bool{}
	for _, e := range events {
		if _, ok := inserted[e.EventID]; !ok || done[e.EventID] {
			continue
		}
		done[e.EventID] = true
		payload, err := json.Marshal(newStoredMessage(e))
		if err != nil {
			return fmt.Errorf("encode outbox message: %w", err)
		}
		keys = append(keys, e.Customer.CustomerID)
		payloads = append(payloads, string(payload))
	}
	if len(keys) == 0 {
		return nil
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox (topic, message_key, payload, headers)
		SELECT $1, t.message_key, t.payload::jsonb, $4::jsonb
		FROM unnest($2::text[], $3::text[]) WITH ORDINALITY AS t(message_key, payload, position)
		ORDER BY t.position`,
		s.topic, keys, payloads, string(headers))
	if err != nil {
		return fmt.Errorf("insert outbox messages: %w", err)
	}
	return nil
}

const purchaseColumns = `
	p.event_id, p.order_id, p.event_timestamp, p.status, p.channel, p.payment_method, p.installments,
	p.currency, p.subtotal::text, p.discount::text, p.shipping::text, p.total::text, p.source, p.received_at,
	c.customer_id, c.name, c.email, c.city, c.state`

// List returns one page of the purchases that match the filter.
func (s *Store) List(ctx context.Context, f ListFilter) (Page[Purchase], error) {
	page := Page[Purchase]{Data: []Purchase{}, Limit: f.Limit, Offset: f.Offset}

	var where whereBuilder
	where.addIf(f.Status != "", "p.status = ?", f.Status)
	where.addIf(f.Channel != "", "p.channel = ?", f.Channel)
	where.addIf(f.PaymentMethod != "", "p.payment_method = ?", f.PaymentMethod)
	where.addIf(f.Source != "", "p.source = ?", f.Source)
	where.addIf(f.CustomerID != "", "p.customer_id = ?", f.CustomerID)
	where.addIf(f.Search != "", "(p.order_id ILIKE ? OR c.name ILIKE ? OR c.email ILIKE ?)", containsPattern(f.Search))
	where.addIf(f.From != nil, "p.event_timestamp >= ?", f.From)
	where.addIf(f.To != nil, "p.event_timestamp < ?", f.To)
	where.addIf(f.MinTotal != "", "p.total >= ?::numeric", f.MinTotal)
	where.addIf(f.MaxTotal != "", "p.total <= ?::numeric", f.MaxTotal)

	from := `FROM purchases p JOIN customers c ON c.customer_id = p.customer_id ` + where.sql()
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+from, where.args...).Scan(&page.Total); err != nil {
		return page, fmt.Errorf("count purchases: %w", err)
	}

	sortExpr, ok := PurchaseSortFields[f.Sort]
	if !ok {
		sortExpr = PurchaseSortFields["event_timestamp"]
	}
	direction := sortDirection(f.Desc)
	args := append(where.args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s %s ORDER BY %s %s, p.event_id %s LIMIT $%d OFFSET $%d`,
		purchaseColumns, from, sortExpr, direction, direction, len(args)-1, len(args)),
		args...)
	if err != nil {
		return page, fmt.Errorf("list purchases: %w", err)
	}
	purchases, err := pgx.CollectRows(rows, scanPurchase)
	if err != nil {
		return page, fmt.Errorf("list purchases: %w", err)
	}
	if err := s.loadItems(ctx, purchases); err != nil {
		return page, err
	}
	page.Data = purchases
	return page, nil
}

// Get returns a single purchase, or ErrNotFound.
func (s *Store) Get(ctx context.Context, eventID uuid.UUID) (Purchase, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+purchaseColumns+`
		FROM purchases p
		JOIN customers c ON c.customer_id = p.customer_id
		WHERE p.event_id = $1`,
		eventID)
	if err != nil {
		return Purchase{}, fmt.Errorf("get purchase: %w", err)
	}
	purchase, err := pgx.CollectExactlyOneRow(rows, scanPurchase)
	if errors.Is(err, pgx.ErrNoRows) {
		return Purchase{}, ErrNotFound
	}
	if err != nil {
		return Purchase{}, fmt.Errorf("get purchase: %w", err)
	}
	purchases := []Purchase{purchase}
	if err := s.loadItems(ctx, purchases); err != nil {
		return Purchase{}, err
	}
	return purchases[0], nil
}

func scanPurchase(row pgx.CollectableRow) (Purchase, error) {
	var (
		p                                   Purchase
		subtotal, discount, shipping, total string
	)
	err := row.Scan(
		&p.EventID, &p.OrderID, &p.EventTimestamp, &p.Status, &p.Channel, &p.PaymentMethod, &p.Installments,
		&p.Currency, &subtotal, &discount, &shipping, &total, &p.Source, &p.ReceivedAt,
		&p.Customer.CustomerID, &p.Customer.Name, &p.Customer.Email, &p.Customer.City, &p.Customer.State,
	)
	p.Subtotal, p.Discount, p.Shipping, p.Total = json.Number(subtotal), json.Number(discount), json.Number(shipping), json.Number(total)
	return p, err
}

// loadItems fills the items of the given purchases with a single query.
func (s *Store) loadItems(ctx context.Context, purchases []Purchase) error {
	if len(purchases) == 0 {
		return nil
	}
	index := make(map[uuid.UUID]int, len(purchases))
	ids := make([]uuid.UUID, len(purchases))
	for i, p := range purchases {
		index[p.EventID] = i
		ids[i] = p.EventID
		purchases[i].Items = []Item{}
	}

	rows, err := s.pool.Query(ctx, `
		SELECT event_id, sku, name, category, unit_price::text, quantity, subtotal::text
		FROM purchase_items
		WHERE event_id = ANY($1)
		ORDER BY id`,
		ids)
	if err != nil {
		return fmt.Errorf("load purchase items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			eventID             uuid.UUID
			item                Item
			unitPrice, subtotal string
		)
		if err := rows.Scan(&eventID, &item.SKU, &item.Name, &item.Category, &unitPrice, &item.Quantity, &subtotal); err != nil {
			return fmt.Errorf("load purchase items: %w", err)
		}
		item.UnitPrice, item.Subtotal = json.Number(unitPrice), json.Number(subtotal)
		i := index[eventID]
		purchases[i].Items = append(purchases[i].Items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("load purchase items: %w", err)
	}
	return nil
}
