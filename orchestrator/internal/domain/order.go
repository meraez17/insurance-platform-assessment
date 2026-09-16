package domain

import (
	"errors"
	"time"
)

type OrderStatus string

const (
	Created         OrderStatus = "CREATED"
	Quoted          OrderStatus = "QUOTED"
	PaymentPending  OrderStatus = "PAYMENT_PENDING"
	Paid            OrderStatus = "PAID"
	Issuing         OrderStatus = "ISSUING"
	Issued          OrderStatus = "ISSUED"
	Failed          OrderStatus = "FAILED"
	RefundRequired  OrderStatus = "REFUND_REQUIRED"
)

var allowedTransitions = map[OrderStatus]map[OrderStatus]bool{
	Created:        {Quoted: true, Failed: true},
	Quoted:         {PaymentPending: true, Failed: true},
	PaymentPending: {Paid: true, Failed: true},
	Paid:           {Issuing: true},
	Issuing:        {Issued: true, RefundRequired: true},
}

type Order struct {
	ID             string      `json:"id"`
	ExternalRef    string      `json:"externalRef"`
	Status         OrderStatus `json:"status"`
	MaskedPlate    string      `json:"maskedPlate"`
	PremiumCents   int64       `json:"premiumCents,omitempty"`
	PolicyNumber   string      `json:"policyNumber,omitempty"`
	CorrelationID  string      `json:"correlationId"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

func (o *Order) Transition(next OrderStatus) error {
	if o.Status == next {
		return nil
	}
	if !allowedTransitions[o.Status][next] {
		return errors.New("invalid order state transition")
	}
	o.Status = next
	o.UpdatedAt = time.Now().UTC()
	return nil
}

func MaskPlate(plate string) string {
	if len(plate) <= 2 {
		return "**"
	}
	return plate[:1] + "***" + plate[len(plate)-1:]
}

