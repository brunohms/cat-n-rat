package stats

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Apply adds a purchase to the stats of its customer and reports whether it was applied. A purchase that was
// already counted (Kafka delivers at least once) is skipped: the event_id is recorded in customer_stats_events
// in the same transaction as the update.
func (s *Store) Apply(ctx context.Context, m Message) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `INSERT INTO customer_stats_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, m.EventID)
	if err != nil {
		return false, fmt.Errorf("record event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	// Every update is commutative (sum, count, max), so the order in which the messages arrive does not matter
	_, err = tx.Exec(ctx, `
		INSERT INTO customer_stats (customer_id, purchase_count, total_spent, last_purchase_at)
		VALUES ($1, 1, $2::numeric, $3)
		ON CONFLICT (customer_id) DO UPDATE SET
			purchase_count = customer_stats.purchase_count + 1,
			total_spent = customer_stats.total_spent + EXCLUDED.total_spent,
			last_purchase_at = GREATEST(customer_stats.last_purchase_at, EXCLUDED.last_purchase_at),
			updated_at = now()`,
		m.CustomerID, m.spent(), m.EventTimestamp)
	if err != nil {
		return false, fmt.Errorf("update customer stats: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}
