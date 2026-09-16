package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"assessment/orchestrator/internal/domain"
	"assessment/orchestrator/internal/storage"
	"assessment/orchestrator/internal/workers"
	amqp "github.com/rabbitmq/amqp091-go"
)

type server struct {
	repo   *storage.Repository
	secret []byte
}

type createOrderRequest struct {
	Plate string `json:"plate"`
}

type paymentEvent struct {
	EventID string `json:"eventId"`
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
}

func main() {
	ctx := context.Background()
	repo, err := storage.New(ctx, env("DATABASE_URL", "postgres://insurance:insurance@localhost:5432/insurance?sslmode=disable"))
	if err != nil { slog.Error("database unavailable", "error", err); os.Exit(1) }
	defer repo.Close()
	broker, err := amqp.Dial(env("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil { slog.Error("broker unavailable", "error", err); os.Exit(1) }
	defer broker.Close()
	channel, err := broker.Channel(); if err != nil { slog.Error("broker channel unavailable", "error", err); os.Exit(1) }
	defer channel.Close()
	processorChannel, err := broker.Channel(); if err != nil { slog.Error("processor channel unavailable", "error", err); os.Exit(1) }
	defer processorChannel.Close()
	_ = channel.ExchangeDeclare("insurance.events", "topic", true, false, false, false, nil)
	_ = channel.ExchangeDeclare("insurance.dlx", "topic", true, false, false, false, nil)
	go workers.NewOutboxPublisher(repo.Pool(), channel).Run(ctx)
	go workers.NewInboxReplayer(repo).Run(ctx)
	go func(){ if runErr:=workers.NewProcessor(repo,processorChannel,env("SIMULATOR_URL","http://localhost:8082"),env("ISSUER_URL","http://localhost:8081")).Run(ctx);runErr!=nil{slog.Error("processor stopped","error",runErr)} }()
	s := &server{repo: repo, secret: []byte(env("WEBHOOK_HMAC_SECRET", "local-development-secret"))}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /orders", s.createOrder)
	mux.HandleFunc("GET /orders", s.listOrders)
	mux.HandleFunc("POST /orders/{id}/retry-issuance", s.retryIssuance)
	mux.HandleFunc("POST /orders/{id}/mark-refund-required", s.markRefundRequired)
	mux.HandleFunc("POST /webhooks/payments", s.paymentWebhook)

	addr := ":8080"
	slog.Info("orchestrator listening", "addr", addr)
	if err := http.ListenAndServe(addr, correlationMiddleware(mux)); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (s *server) createOrder(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Idempotency-Key is required"})
		return
	}
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Plate) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid plate is required"})
		return
	}
	now := time.Now().UTC()
	id := "ord_" + now.Format("20060102150405.000000000")
	o := domain.Order{ID: id, ExternalRef: id, Status: domain.Created, MaskedPlate: domain.MaskPlate(req.Plate), CorrelationID: correlationID(r), CreatedAt: now, UpdatedAt: now}
	bodyHash := storage.Hash(strings.ToUpper(strings.TrimSpace(req.Plate)))
	created, replayed, err := s.repo.CreateOrder(r.Context(), storage.Hash(key), bodyHash, o)
	if errors.Is(err, storage.ErrIdempotencyConflict) { writeJSON(w,http.StatusConflict,map[string]string{"error":err.Error()}); return }
	if err != nil { slog.Error("create order failed","error",err,"correlationId",correlationID(r)); writeJSON(w,http.StatusInternalServerError,map[string]string{"error":"order could not be created"}); return }
	if replayed { writeJSON(w,http.StatusOK,created); return }
	writeJSON(w, http.StatusAccepted, created)
}

func (s *server) listOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.repo.List(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("q"))
	if err != nil { writeJSON(w,http.StatusInternalServerError,map[string]string{"error":"orders could not be loaded"}); return }
	writeJSON(w, http.StatusOK, orders)
}

func (s *server) retryIssuance(w http.ResponseWriter, r *http.Request) {
	actor := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actor == "" { writeJSON(w,http.StatusBadRequest,map[string]string{"error":"X-Actor-ID is required"}); return }
	id := r.PathValue("id")
	order, err := s.repo.Get(r.Context(),id)
	if err != nil || order.Status != domain.Issuing { writeJSON(w,http.StatusConflict,map[string]string{"error":"only ISSUING orders can be retried"}); return }
	updated, err := s.repo.Transition(r.Context(),id,domain.Issuing,domain.Issuing,"ISSUANCE_RETRY_REQUESTED",map[string]any{"orderId":id,"actor":actor})
	if err != nil { writeJSON(w,http.StatusConflict,map[string]string{"error":err.Error()}); return }
	writeJSON(w,http.StatusAccepted,updated)
}

func (s *server) markRefundRequired(w http.ResponseWriter, r *http.Request) {
	actor := strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	if actor == "" { writeJSON(w,http.StatusBadRequest,map[string]string{"error":"X-Actor-ID is required"}); return }
	id := r.PathValue("id")
	updated, err := s.repo.Transition(r.Context(),id,domain.Issuing,domain.RefundRequired,"REFUND_REVIEW_REQUESTED",map[string]any{"actor":actor})
	if err != nil { writeJSON(w,http.StatusConflict,map[string]string{"error":err.Error()}); return }
	writeJSON(w,http.StatusAccepted,updated)
}

func (s *server) paymentWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil || !validSignature(s.secret, body, r.Header.Get("X-Signature")) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}
	var event paymentEvent
	if err := json.Unmarshal(body, &event); err != nil || event.EventID == "" || event.OrderID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event"})
		return
	}
	inserted, err := s.repo.SaveInbox(r.Context(), event.EventID, "PAYMENT_"+event.Status, body)
	if err != nil { writeJSON(w,http.StatusInternalServerError,map[string]string{"error":"event could not be persisted"}); return }
	if !inserted {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	order, err := s.repo.Get(r.Context(), event.OrderID)
	if err != nil || order.Status != domain.PaymentPending {
		_ = s.repo.DeferInbox(r.Context(), event.EventID, "order not ready", time.Second)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "deferred"})
		return
	}
	if event.Status == "APPROVED" && order.Status == domain.PaymentPending {
		if _, err := s.repo.Transition(r.Context(),order.ID,domain.PaymentPending,domain.Paid,"PAYMENT_APPROVED",map[string]any{"orderId":order.ID,"eventId":event.EventID}); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
	}
	_ = s.repo.MarkInboxProcessed(r.Context(),event.EventID)
	w.WriteHeader(http.StatusNoContent)
}

func validSignature(secret, payload []byte, received string) bool {
	decoded, err := hex.DecodeString(received)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	return hmac.Equal(mac.Sum(nil), decoded)
}

func correlationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Correlation-ID"))
		if id == "" {
			id = "cor_" + time.Now().UTC().Format("20060102150405.000000000")
		}
		r.Header.Set("X-Correlation-ID", id)
		w.Header().Set("X-Correlation-ID", id)
		next.ServeHTTP(w, r)
	})
}

func correlationID(r *http.Request) string { return r.Header.Get("X-Correlation-ID") }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
