package core

import (
	"errors"
	"sort"
	"time"
)

var ErrInsufficientInventory = errors.New("insufficient inventory")

type Batch struct {
	ID         string
	Initial    Quantity
	Current    Quantity
	Expiry     *time.Time
	PriceCents int64
	CreatedAt  time.Time
}

type Allocation struct {
	BatchID     string   `json:"batchId"`
	Quantity    Quantity `json:"-"`
	UnitCostCNY float64  `json:"unitCostCny"`
}

func AllocateFEFO(batches []Batch, requested Quantity) ([]Allocation, error) {
	if requested <= 0 {
		return nil, errors.New("requested quantity must be positive")
	}
	total := Quantity(0)
	for _, batch := range batches {
		if batch.Current > 0 {
			total += batch.Current
		}
	}
	if total < requested {
		return nil, ErrInsufficientInventory
	}
	sort.SliceStable(batches, func(i, j int) bool {
		left, right := batches[i], batches[j]
		if left.Expiry == nil && right.Expiry != nil {
			return false
		}
		if left.Expiry != nil && right.Expiry == nil {
			return true
		}
		if left.Expiry != nil && right.Expiry != nil && !left.Expiry.Equal(*right.Expiry) {
			return left.Expiry.Before(*right.Expiry)
		}
		return left.CreatedAt.Before(right.CreatedAt)
	})
	remaining := requested
	allocations := make([]Allocation, 0)
	for index := range batches {
		if remaining <= 0 {
			break
		}
		if batches[index].Current <= 0 {
			continue
		}
		quantity := batches[index].Current
		if quantity > remaining {
			quantity = remaining
		}
		batches[index].Current -= quantity
		remaining -= quantity
		unitCost := 0.0
		if batches[index].Initial > 0 {
			unitCost = float64(batches[index].PriceCents) / 100 / batches[index].Initial.Float64()
		}
		allocations = append(allocations, Allocation{BatchID: batches[index].ID, Quantity: quantity, UnitCostCNY: unitCost})
	}
	return allocations, nil
}

func RestoreExact(batches []Batch, allocations []Allocation) error {
	byID := make(map[string]*Batch, len(batches))
	for index := range batches {
		byID[batches[index].ID] = &batches[index]
	}
	for _, allocation := range allocations {
		batch := byID[allocation.BatchID]
		if batch == nil {
			return errors.New("allocation batch does not exist")
		}
		if allocation.Quantity <= 0 || batch.Current+allocation.Quantity > batch.Initial {
			return errors.New("allocation cannot be restored safely")
		}
	}
	for _, allocation := range allocations {
		byID[allocation.BatchID].Current += allocation.Quantity
	}
	return nil
}
