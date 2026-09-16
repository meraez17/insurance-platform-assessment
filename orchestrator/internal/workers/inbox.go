package workers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"assessment/orchestrator/internal/domain"
	"assessment/orchestrator/internal/storage"
)

type InboxReplayer struct{ repo *storage.Repository }
func NewInboxReplayer(repo *storage.Repository)*InboxReplayer{return &InboxReplayer{repo:repo}}

func(r *InboxReplayer)Run(ctx context.Context){ticker:=time.NewTicker(time.Second);defer ticker.Stop();for{select{case<-ctx.Done():return;case<-ticker.C:r.replay(ctx)}}}
func(r *InboxReplayer)replay(ctx context.Context){messages,err:=r.repo.DeferredInbox(ctx,20);if err!=nil{slog.Error("load deferred inbox","error",err);return};for _,message:=range messages{var event struct{OrderID string `json:"orderId"`;Status string `json:"status"`};if json.Unmarshal(message.Payload,&event)!=nil{_ = r.repo.DeferInbox(ctx,message.ID,"invalid payload",time.Hour);continue};order,loadErr:=r.repo.Get(ctx,event.OrderID);if loadErr!=nil||order.Status!=domain.PaymentPending{_ = r.repo.DeferInbox(ctx,message.ID,"order not ready",backoff(message.Attempts+1));continue};if event.Status=="APPROVED"{if _,transitionErr:=r.repo.Transition(ctx,event.OrderID,domain.PaymentPending,domain.Paid,"PAYMENT_APPROVED",map[string]any{"orderId":event.OrderID,"eventId":message.ID});transitionErr!=nil{_ = r.repo.DeferInbox(ctx,message.ID,transitionErr.Error(),backoff(message.Attempts+1));continue}};_ = r.repo.MarkInboxProcessed(ctx,message.ID)}}
