-- Transactional outbox: one row per message to publish to Kafka, written in the same transaction as the purchases,
-- so a stored purchase is never left without its message. The backend relays the rows to Kafka and deletes them.
CREATE TABLE outbox (
    id          BIGSERIAL    PRIMARY KEY,
    topic       VARCHAR(249) NOT NULL,
    message_key VARCHAR(200) NOT NULL,
    payload     JSONB        NOT NULL,
    -- Trace context of the request that stored the purchase (W3C traceparent), copied to the Kafka headers
    headers     JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Purchase aggregates per customer, maintained by the worker from the Kafka messages
CREATE TABLE customer_stats (
    customer_id      VARCHAR(20)   PRIMARY KEY REFERENCES customers (customer_id),
    purchase_count   INTEGER       NOT NULL DEFAULT 0,
    -- Sum of the totals of the purchases with status "paid"
    total_spent      NUMERIC(14,2) NOT NULL DEFAULT 0,
    last_purchase_at TIMESTAMPTZ,
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- Purchases already counted in customer_stats: Kafka delivers at least once, so a redelivered message is skipped
CREATE TABLE customer_stats_events (
    event_id     UUID        PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Purchases stored before this migration have no message, so they are counted here
INSERT INTO customer_stats (customer_id, purchase_count, total_spent, last_purchase_at)
SELECT customer_id, count(*), coalesce(sum(total) FILTER (WHERE status = 'paid'), 0), max(event_timestamp)
FROM purchases
GROUP BY customer_id;

INSERT INTO customer_stats_events (event_id)
SELECT event_id FROM purchases;
