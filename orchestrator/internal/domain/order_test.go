package domain

import "testing"

func TestOrderTransitionRejectsSkippingPayment(t *testing.T) {
	o := Order{Status: PaymentPending}
	if err := o.Transition(Issued); err == nil {
		t.Fatal("expected invalid transition to be rejected")
	}
}

func TestOrderTransitionIsIdempotent(t *testing.T) {
	o := Order{Status: Paid}
	if err := o.Transition(Paid); err != nil {
		t.Fatalf("same-state transition must be idempotent: %v", err)
	}
}

func TestMaskPlate(t *testing.T) {
	if got := MaskPlate("ABC123"); got != "A***3" {
		t.Fatalf("unexpected mask: %s", got)
	}
}

