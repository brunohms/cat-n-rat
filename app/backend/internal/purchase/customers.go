package purchase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CustomerSummary is a customer with aggregates of their purchases.
type CustomerSummary struct {
	Customer
	PurchaseCount int `json:"purchase_count"`
	// Sum of the totals of the purchases with status "paid"
	TotalSpent     json.Number `json:"total_spent"`
	LastPurchaseAt *time.Time  `json:"last_purchase_at"`
}

// CustomerFilter selects customers; empty fields are ignored.
type CustomerFilter struct {
	Limit  int
	Offset int
	// Case-insensitive match on customer_id, name or email
	Search string
	State  string
	City   string
	// One of CustomerSortFields; empty means name
	Sort string
	Desc bool
}

// CustomerSortFields maps the accepted sort keys to their SQL expressions.
var CustomerSortFields = map[string]string{
	"name":             "c.name",
	"customer_id":      "c.customer_id",
	"state":            "c.state",
	"purchase_count":   "count(p.event_id)",
	"total_spent":      totalSpentExpr,
	"last_purchase_at": "max(p.event_timestamp)",
}

const totalSpentExpr = "coalesce(sum(p.total) FILTER (WHERE p.status = 'paid'), 0)"

// ListCustomers returns one page of the customers that match the filter.
func (s *Store) ListCustomers(ctx context.Context, f CustomerFilter) (Page[CustomerSummary], error) {
	page := Page[CustomerSummary]{Data: []CustomerSummary{}, Limit: f.Limit, Offset: f.Offset}

	var where whereBuilder
	where.addIf(f.Search != "", "(c.customer_id ILIKE ? OR c.name ILIKE ? OR c.email ILIKE ?)", containsPattern(f.Search))
	where.addIf(f.State != "", "c.state = ?", f.State)
	where.addIf(f.City != "", "c.city ILIKE ?", containsPattern(f.City))

	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM customers c `+where.sql(), where.args...).Scan(&page.Total); err != nil {
		return page, fmt.Errorf("count customers: %w", err)
	}

	sortExpr, ok := CustomerSortFields[f.Sort]
	if !ok {
		sortExpr = CustomerSortFields["name"]
	}
	direction := sortDirection(f.Desc)
	args := append(where.args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT c.customer_id, c.name, c.email, c.city, c.state,
			count(p.event_id), `+totalSpentExpr+`::text, max(p.event_timestamp)
		FROM customers c
		LEFT JOIN purchases p ON p.customer_id = c.customer_id
		%s
		GROUP BY c.customer_id
		ORDER BY %s %s NULLS LAST, c.customer_id %s
		LIMIT $%d OFFSET $%d`,
		where.sql(), sortExpr, direction, direction, len(args)-1, len(args)),
		args...)
	if err != nil {
		return page, fmt.Errorf("list customers: %w", err)
	}
	customers, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CustomerSummary, error) {
		var (
			c          CustomerSummary
			totalSpent string
		)
		err := row.Scan(&c.CustomerID, &c.Name, &c.Email, &c.City, &c.State, &c.PurchaseCount, &totalSpent, &c.LastPurchaseAt)
		c.TotalSpent = json.Number(totalSpent)
		return c, err
	})
	if err != nil {
		return page, fmt.Errorf("list customers: %w", err)
	}
	page.Data = customers
	return page, nil
}

// FilterOptions are the distinct values available for the list filters.
type FilterOptions struct {
	Statuses       []string `json:"statuses"`
	Channels       []string `json:"channels"`
	PaymentMethods []string `json:"payment_methods"`
	Sources        []string `json:"sources"`
	States         []string `json:"states"`
}

func (s *Store) FilterOptions(ctx context.Context) (FilterOptions, error) {
	var o FilterOptions
	err := s.pool.QueryRow(ctx, `
		SELECT
			array(SELECT DISTINCT status FROM purchases ORDER BY 1),
			array(SELECT DISTINCT channel FROM purchases ORDER BY 1),
			array(SELECT DISTINCT payment_method FROM purchases ORDER BY 1),
			array(SELECT DISTINCT source FROM purchases ORDER BY 1),
			array(SELECT DISTINCT state FROM customers ORDER BY 1)`,
	).Scan(&o.Statuses, &o.Channels, &o.PaymentMethods, &o.Sources, &o.States)
	if err != nil {
		return o, fmt.Errorf("load filter options: %w", err)
	}
	return o, nil
}
