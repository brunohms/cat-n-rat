CREATE TABLE customers (
    customer_id VARCHAR(20) PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    email       VARCHAR(320) NOT NULL,
    city        VARCHAR(120) NOT NULL,
    state       VARCHAR(60)  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE purchases (
    event_id        UUID PRIMARY KEY,
    order_id        VARCHAR(20)   NOT NULL,
    event_timestamp TIMESTAMPTZ   NOT NULL,
    status          VARCHAR(20)   NOT NULL,
    channel         VARCHAR(20)   NOT NULL,
    customer_id     VARCHAR(20)   NOT NULL REFERENCES customers (customer_id),
    payment_method  VARCHAR(20)   NOT NULL,
    installments    INTEGER       NOT NULL,
    currency        CHAR(3)       NOT NULL,
    subtotal        NUMERIC(12,2) NOT NULL,
    discount        NUMERIC(12,2) NOT NULL,
    shipping        NUMERIC(12,2) NOT NULL,
    total           NUMERIC(12,2) NOT NULL,
    -- How the event arrived: 'push' (sent by the generator) or 'pull' (requested from it)
    source          VARCHAR(10)   NOT NULL CHECK (source IN ('push', 'pull')),
    -- Original event as received, kept for auditing and reprocessing
    payload         JSONB         NOT NULL,
    received_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX ix_purchases_order_id ON purchases (order_id);
CREATE INDEX ix_purchases_event_timestamp ON purchases (event_timestamp);
CREATE INDEX ix_purchases_status ON purchases (status);
CREATE INDEX ix_purchases_customer_id ON purchases (customer_id);

CREATE TABLE purchase_items (
    id         BIGSERIAL PRIMARY KEY,
    event_id   UUID          NOT NULL REFERENCES purchases (event_id) ON DELETE CASCADE,
    sku        VARCHAR(40)   NOT NULL,
    name       VARCHAR(200)  NOT NULL,
    category   VARCHAR(40)   NOT NULL,
    unit_price NUMERIC(12,2) NOT NULL,
    quantity   INTEGER       NOT NULL CHECK (quantity >= 1),
    subtotal   NUMERIC(12,2) NOT NULL
);

CREATE INDEX ix_purchase_items_event_id ON purchase_items (event_id);
CREATE INDEX ix_purchase_items_sku ON purchase_items (sku);
