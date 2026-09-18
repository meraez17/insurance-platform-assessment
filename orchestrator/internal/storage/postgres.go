package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"assessment/orchestrator/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIdempotencyConflict = errors.New("idempotency key reused with a different request")

type Repository struct{ pool *pgxpool.Pool }

type OrderEvent struct {
	ID         string          `json:"id"`
	OrderID    string          `json:"orderId"`
	EventType  string          `json:"eventType"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurredAt"`
}

func New(ctx context.Context, databaseURL string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close()              { r.pool.Close() }
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

func (r *Repository) Get(ctx context.Context, id string) (domain.Order, error) {
	return scanOrder(r.pool.QueryRow(ctx, orderSelect+` WHERE id=$1`, id))
}

func (r *Repository) List(ctx context.Context, status, query string) ([]domain.Order, error) {
	rows, err := r.pool.Query(ctx, orderSelect+`
		WHERE ($1='' OR status=$1) AND ($2='' OR id ILIKE '%'||$2||'%' OR external_ref ILIKE '%'||$2||'%')
		ORDER BY created_at DESC LIMIT 200`, status, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]domain.Order, 0)
	for rows.Next() {
		order, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *Repository) ListEvents(ctx context.Context, orderID string) ([]OrderEvent, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,order_id,event_type,payload,occurred_at FROM order_events WHERE order_id=$1 ORDER BY occurred_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]OrderEvent, 0)
	for rows.Next() {
		var event OrderEvent
		if err = rows.Scan(&event.ID, &event.OrderID, &event.EventType, &event.Payload, &event.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) MarkInboxProcessed(ctx context.Context, messageID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE inbox_messages SET status='PROCESSED',processed_at=now() WHERE message_id=$1`, messageID)
	return err
}

type InboxMessage struct {
	ID       string
	Type     string
	Payload  []byte
	Attempts int
}

func (r *Repository) DeferredInbox(ctx context.Context, limit int) ([]InboxMessage, error) {
	rows, err := r.pool.Query(ctx, `SELECT message_id,message_type,payload,attempts FROM inbox_messages WHERE status='DEFERRED' AND available_at<=now() ORDER BY received_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]InboxMessage, 0)
	for rows.Next() {
		var message InboxMessage
		if err = rows.Scan(&message.ID, &message.Type, &message.Payload, &message.Attempts); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (r *Repository) CreateOrder(ctx context.Context, keyHash, requestHash string, order domain.Order) (domain.Order, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Order{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existingRequestHash, existingOrderID string
	err = tx.QueryRow(ctx, `SELECT request_hash, order_id FROM idempotency_keys WHERE key_hash=$1`, keyHash).Scan(&existingRequestHash, &existingOrderID)
	if err == nil {
		if existingRequestHash != requestHash {
			return domain.Order{}, false, ErrIdempotencyConflict
		}
		existing, getErr := getOrder(ctx, tx, existingOrderID)
		return existing, true, getErr
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, false, err
	}

	_, err = tx.Exec(ctx, `INSERT INTO orders(id,external_ref,status,masked_plate,premium_cents,policy_number,correlation_id,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, order.ID, order.ExternalRef, order.Status, order.MaskedPlate, order.PremiumCents, nullable(order.PolicyNumber), order.CorrelationID, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return domain.Order{}, false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO idempotency_keys(key_hash,request_hash,order_id) VALUES($1,$2,$3)`, keyHash, requestHash, order.ID)
	if err != nil {
		return domain.Order{}, false, err
	}
	if err = appendEventAndOutbox(ctx, tx, order, "ORDER_CREATED", map[string]any{"orderId": order.ID}); err != nil {
		return domain.Order{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Order{}, false, err
	}
	return order, false, nil
}

func (r *Repository) Transition(ctx context.Context, orderID string, expected, next domain.OrderStatus, eventType string, payload map[string]any) (domain.Order, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.Status != expected {
		return domain.Order{}, fmt.Errorf("expected %s, found %s", expected, order.Status)
	}
	if err = order.Transition(next); err != nil {
		return domain.Order{}, err
	}
	result, err := tx.Exec(ctx, `UPDATE orders SET status=$1,updated_at=$2,version=version+1 WHERE id=$3 AND status=$4`, next, order.UpdatedAt, order.ID, expected)
	if err != nil {
		return domain.Order{}, err
	}
	if result.RowsAffected() != 1 {
		return domain.Order{}, errors.New("concurrent order update")
	}
	if err = appendEventAndOutbox(ctx, tx, order, eventType, payload); err != nil {
		return domain.Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *Repository) ApplyQuote(ctx context.Context, orderID string, premium int64) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.Status != domain.Created {
		return domain.Order{}, fmt.Errorf("expected CREATED, found %s", order.Status)
	}
	if err = order.Transition(domain.Quoted); err != nil {
		return domain.Order{}, err
	}
	order.PremiumCents = premium
	_, err = tx.Exec(ctx, `UPDATE orders SET status=$1,premium_cents=$2,updated_at=$3,version=version+1 WHERE id=$4`, order.Status, premium, order.UpdatedAt, order.ID)
	if err != nil {
		return domain.Order{}, err
	}
	if err = appendEventAndOutbox(ctx, tx, order, "ORDER_QUOTED", map[string]any{"premiumCents": premium}); err != nil {
		return domain.Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *Repository) MarkIssued(ctx context.Context, orderID, policyNumber string) (domain.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	order, err := getOrderForUpdate(ctx, tx, orderID)
	if err != nil {
		return domain.Order{}, err
	}
	if order.Status != domain.Issuing {
		return domain.Order{}, fmt.Errorf("expected ISSUING, found %s", order.Status)
	}
	if err = order.Transition(domain.Issued); err != nil {
		return domain.Order{}, err
	}
	order.PolicyNumber = policyNumber
	_, err = tx.Exec(ctx, `UPDATE orders SET status=$1,policy_number=$2,updated_at=$3,version=version+1 WHERE id=$4`, order.Status, policyNumber, order.UpdatedAt, order.ID)
	if err != nil {
		return domain.Order{}, err
	}
	if err = appendEventAndOutbox(ctx, tx, order, "POLICY_ISSUED", map[string]any{"policyNumber": policyNumber}); err != nil {
		return domain.Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}

func (r *Repository) SaveInbox(ctx context.Context, messageID, messageType string, payload []byte) (bool, error) {
	result, err := r.pool.Exec(ctx, `INSERT INTO inbox_messages(message_id,message_type,payload) VALUES($1,$2,$3)
		ON CONFLICT(message_id) DO NOTHING`, messageID, messageType, payload)
	return result.RowsAffected() == 1, err
}

func (r *Repository) DeferInbox(ctx context.Context, messageID, reason string, delay time.Duration) error {
	_, err := r.pool.Exec(ctx, `UPDATE inbox_messages SET status='DEFERRED',attempts=attempts+1,last_error=$2,available_at=$3 WHERE message_id=$1`, messageID, reason, time.Now().UTC().Add(delay))
	return err
}

func appendEventAndOutbox(ctx context.Context, tx pgx.Tx, order domain.Order, eventType string, payload map[string]any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `WITH event AS (SELECT gen_random_uuid() id), outbox AS (SELECT gen_random_uuid() id)
		INSERT INTO order_events(id,order_id,event_type,payload) SELECT event.id,$1,$2,$3 FROM event`, order.ID, eventType, data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox_messages(id,aggregate_id,event_type,payload,correlation_id) VALUES(gen_random_uuid(),$1,$2,$3,$4)`, order.ID, eventType, data, order.CorrelationID)
	return err
}

type rowScanner interface{ Scan(dest ...any) error }

func getOrder(ctx context.Context, tx pgx.Tx, id string) (domain.Order, error) {
	return scanOrder(tx.QueryRow(ctx, orderSelect+` WHERE id=$1`, id))
}
func getOrderForUpdate(ctx context.Context, tx pgx.Tx, id string) (domain.Order, error) {
	return scanOrder(tx.QueryRow(ctx, orderSelect+` WHERE id=$1 FOR UPDATE`, id))
}

const orderSelect = `SELECT id,external_ref,status,masked_plate,COALESCE(premium_cents,0),COALESCE(policy_number,''),correlation_id,created_at,updated_at FROM orders`

func scanOrder(row rowScanner) (domain.Order, error) {
	var o domain.Order
	err := row.Scan(&o.ID, &o.ExternalRef, &o.Status, &o.MaskedPlate, &o.PremiumCents, &o.PolicyNumber, &o.CorrelationID, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
