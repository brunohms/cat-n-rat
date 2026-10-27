package api

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/brunohms/cat-n-rat/app/backend/internal/purchase"
)

// listPurchases returns one page of purchases matching the query string filters.
func (s *Server) listPurchases(w http.ResponseWriter, r *http.Request) {
	q := queryParser{values: r.URL.Query()}
	filter := purchase.ListFilter{
		Limit:         q.int("limit", 50, 1, 500),
		Offset:        q.int("offset", 0, 0, -1),
		Status:        q.values.Get("status"),
		Channel:       q.values.Get("channel"),
		PaymentMethod: q.values.Get("payment_method"),
		Source:        q.values.Get("source"),
		CustomerID:    q.values.Get("customer_id"),
		Search:        strings.TrimSpace(q.values.Get("q")),
		From:          q.time("from"),
		To:            q.time("to"),
		MinTotal:      q.decimal("min_total"),
		MaxTotal:      q.decimal("max_total"),
		Sort:          q.oneOf("sort", "event_timestamp", slices.Sorted(maps.Keys(purchase.PurchaseSortFields))),
		Desc:          q.oneOf("order", "desc", []string{"asc", "desc"}) == "desc",
	}
	if q.err != nil {
		writeError(w, http.StatusUnprocessableEntity, q.err.Error())
		return
	}

	page, err := s.store.List(r.Context(), filter)
	if err != nil {
		s.logger.Error("failed to list purchases", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list purchases")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// listCustomers returns one page of customers, with purchase aggregates, matching the query string filters.
func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	q := queryParser{values: r.URL.Query()}
	filter := purchase.CustomerFilter{
		Limit:  q.int("limit", 50, 1, 500),
		Offset: q.int("offset", 0, 0, -1),
		Search: strings.TrimSpace(q.values.Get("q")),
		State:  q.values.Get("state"),
		City:   strings.TrimSpace(q.values.Get("city")),
		Sort:   q.oneOf("sort", "name", slices.Sorted(maps.Keys(purchase.CustomerSortFields))),
		Desc:   q.oneOf("order", "asc", []string{"asc", "desc"}) == "desc",
	}
	if q.err != nil {
		writeError(w, http.StatusUnprocessableEntity, q.err.Error())
		return
	}

	page, err := s.store.ListCustomers(r.Context(), filter)
	if err != nil {
		s.logger.Error("failed to list customers", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list customers")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// filterOptions returns the distinct values available for the select filters.
func (s *Server) filterOptions(w http.ResponseWriter, r *http.Request) {
	options, err := s.store.FilterOptions(r.Context())
	if err != nil {
		s.logger.Error("failed to load filter options", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load filter options")
		return
	}
	writeJSON(w, http.StatusOK, options)
}

// queryParser reads typed query string parameters, keeping the first error.
type queryParser struct {
	values url.Values
	err    error
}

func (q *queryParser) fail(name, message string) {
	if q.err == nil {
		q.err = fmt.Errorf("%s %s", name, message)
	}
}

// int parses an optional integer; max < 0 means no upper bound.
func (q *queryParser) int(name string, fallback, min, max int) int {
	raw := q.values.Get(name)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || (max >= 0 && value > max) {
		if max >= 0 {
			q.fail(name, fmt.Sprintf("must be an integer between %d and %d", min, max))
		} else {
			q.fail(name, fmt.Sprintf("must be an integer greater than or equal to %d", min))
		}
		return fallback
	}
	return value
}

// time parses an optional RFC 3339 timestamp or a YYYY-MM-DD date (midnight UTC).
func (q *queryParser) time(name string) *time.Time {
	raw := q.values.Get(name)
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.DateOnly} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	q.fail(name, "must be an RFC 3339 timestamp or a YYYY-MM-DD date")
	return nil
}

// decimal validates an optional decimal number and returns it unchanged, so it reaches NUMERIC without float rounding.
func (q *queryParser) decimal(name string) string {
	raw := strings.TrimSpace(q.values.Get(name))
	if raw == "" {
		return ""
	}
	if _, err := strconv.ParseFloat(raw, 64); err != nil || strings.ContainsAny(raw, "eExXnN") {
		q.fail(name, "must be a decimal number")
		return ""
	}
	return raw
}

func (q *queryParser) oneOf(name, fallback string, allowed []string) string {
	raw := q.values.Get(name)
	if raw == "" {
		return fallback
	}
	if !slices.Contains(allowed, raw) {
		q.fail(name, "must be one of: "+strings.Join(allowed, ", "))
		return fallback
	}
	return raw
}
