package workers

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type OutboxPublisher struct { db *pgxpool.Pool; channel *amqp.Channel }

func NewOutboxPublisher(db *pgxpool.Pool, channel *amqp.Channel) *OutboxPublisher { return &OutboxPublisher{db: db, channel: channel} }

func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond); defer ticker.Stop()
	for { select { case <-ctx.Done(): return; case <-ticker.C: if err:=p.publishBatch(ctx); err!=nil { slog.Error("outbox batch failed","error",err) } } }
}

func (p *OutboxPublisher) publishBatch(ctx context.Context) error {
	rows, err := p.db.Query(ctx, `WITH claimed AS (
		SELECT id FROM outbox_messages WHERE published_at IS NULL AND available_at<=now()
		ORDER BY created_at LIMIT 20 FOR UPDATE SKIP LOCKED
	) UPDATE outbox_messages o SET available_at=now()+interval '2 minutes'
	FROM claimed WHERE o.id=claimed.id
	RETURNING o.id,o.aggregate_id,o.event_type,o.payload,o.correlation_id,o.attempts`)
	if err != nil { return err }; defer rows.Close()
	type item struct{id,aggregate,eventType,correlation string; payload json.RawMessage; attempts int}
	var items []item
	for rows.Next(){ var i item; if err=rows.Scan(&i.id,&i.aggregate,&i.eventType,&i.payload,&i.correlation,&i.attempts);err!=nil{return err};items=append(items,i) }
	for _,i:=range items {
		err=p.channel.PublishWithContext(ctx,"insurance.events",i.eventType,false,false,amqp.Publishing{MessageId:i.id,CorrelationId:i.correlation,ContentType:"application/json",DeliveryMode:amqp.Persistent,Body:i.payload})
		if err==nil { _,err=p.db.Exec(ctx,`UPDATE outbox_messages SET published_at=now() WHERE id=$1 AND published_at IS NULL`,i.id); if err!=nil{return err}; continue }
		attempts:=i.attempts+1
		if attempts>=8 { _,_ = p.db.Exec(ctx,`UPDATE outbox_messages SET published_at=now(),attempts=$2,last_error=$3 WHERE id=$1`,i.id,attempts,err.Error()); _=p.channel.PublishWithContext(ctx,"insurance.dlx",i.eventType,false,false,amqp.Publishing{MessageId:i.id,CorrelationId:i.correlation,Body:i.payload}); continue }
		delay:=backoff(attempts); _,_ = p.db.Exec(ctx,`UPDATE outbox_messages SET attempts=$2,last_error=$3,available_at=$4 WHERE id=$1`,i.id,attempts,err.Error(),time.Now().UTC().Add(delay))
	}
	return rows.Err()
}

func backoff(attempt int) time.Duration { base:=math.Pow(2,float64(attempt))*250; jitter:=rand.Float64()*250; return time.Duration(base+jitter)*time.Millisecond }
