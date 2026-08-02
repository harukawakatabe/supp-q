package core

import (
	"errors"
	"testing"
	"time"
)

func q(value float64) Quantity            { result, _ := QuantityFromFloat(value); return result }
func datePointer(value string) *time.Time { parsed, _ := ParseDate(value); return &parsed }

func TestFEFOAndExactRestore(t *testing.T) {
	batches := []Batch{{ID: "late", Initial: q(5), Current: q(5), Expiry: datePointer("2027-12-31"), PriceCents: 5000, CreatedAt: mustDate(t, "2026-01-01")}, {ID: "early", Initial: q(3), Current: q(3), Expiry: datePointer("2026-12-31"), PriceCents: 2400, CreatedAt: mustDate(t, "2026-02-01")}}
	allocations, err := AllocateFEFO(batches, q(5))
	if err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 2 || allocations[0].BatchID != "early" || allocations[0].Quantity != q(3) || allocations[1].BatchID != "late" || allocations[1].Quantity != q(2) {
		t.Fatalf("unexpected allocations: %+v", allocations)
	}
	if err = RestoreExact(batches, allocations); err != nil {
		t.Fatal(err)
	}
	for _, batch := range batches {
		if batch.Current != batch.Initial {
			t.Fatalf("batch %s was not restored", batch.ID)
		}
	}
}

func TestAllocationRejectsPartialConsumption(t *testing.T) {
	batches := []Batch{{ID: "only", Initial: q(2), Current: q(2), CreatedAt: time.Now()}}
	_, err := AllocateFEFO(batches, q(3))
	if !errors.Is(err, ErrInsufficientInventory) {
		t.Fatalf("expected insufficient inventory, got %v", err)
	}
	if batches[0].Current != q(2) {
		t.Fatal("failed allocation must not mutate inventory")
	}
}
