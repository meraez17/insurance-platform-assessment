CREATE TABLE IF NOT EXISTS orders (
  id text PRIMARY KEY,
  external_ref text NOT NULL UNIQUE,
  status text NOT NULL CHECK (status IN ('CREATED','QUOTED','PAYMENT_PENDING','PAID','ISSUING','ISSUED','FAILED','REFUND_REQUIRED')),
  masked_plate text NOT NULL,
  premium_cents bigint,
  policy_number text,
  correlation_id text NOT NULL,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  key_hash text PRIMARY KEY,
  request_hash text NOT NULL,
  order_id text NOT NULL REFERENCES orders(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_events (
  id uuid PRIMARY KEY,
  order_id text NOT NULL REFERENCES orders(id),
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_order_events_order_time ON order_events(order_id, occurred_at);

CREATE TABLE IF NOT EXISTS outbox_messages (
  id uuid PRIMARY KEY,
  aggregate_id text NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  correlation_id text NOT NULL,
  attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_outbox_ready ON outbox_messages(available_at) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS inbox_messages (
  message_id text PRIMARY KEY,
  message_type text NOT NULL,
  payload jsonb NOT NULL,
  status text NOT NULL DEFAULT 'RECEIVED' CHECK (status IN ('RECEIVED','DEFERRED','PROCESSED','FAILED')),
  attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL DEFAULT now(),
  last_error text,
  received_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_inbox_deferred ON inbox_messages(available_at) WHERE status = 'DEFERRED';

CREATE TABLE IF NOT EXISTS operation_actions (
  id uuid PRIMARY KEY,
  order_id text NOT NULL REFERENCES orders(id),
  action text NOT NULL,
  actor text NOT NULL,
  reason text NOT NULL,
  result text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

