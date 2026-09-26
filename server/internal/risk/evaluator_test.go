package risk

import (
	"testing"
	"time"

	"suppq.local/server/internal/core"
)

func quantity(t *testing.T, value string) core.Quantity {
	t.Helper()
	result, err := core.ParseQuantity(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func at(t *testing.T, value string, location *time.Location) time.Time {
	t.Helper()
	result, err := time.ParseInLocation("2006-01-02 15:04", value, location)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestEvaluateUsesStableFEFOAndFindsFirstShortfall(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := at(t, "2026-09-26 08:00", location)
	projection, err := Evaluate(Input{
		Now: now, Location: location, PlanState: "active", PlanValid: true,
		RestockThresholdPlanDays: 2, ExpiryReminderDays: 30,
		Batches: []Batch{
			{ID: "unknown", Quantity: quantity(t, "2"), ReceivedAt: now, CreatedAt: now, ExpiryPrecision: ExpiryUnknown},
			{ID: "later", Quantity: quantity(t, "2"), ReceivedAt: now, CreatedAt: now, ExpiryRawValue: "2026-12-31", ExpiryPrecision: ExpiryDay},
			{ID: "earlier", Quantity: quantity(t, "1"), ReceivedAt: now, CreatedAt: now, ExpiryRawValue: "2026-10-31", ExpiryPrecision: ExpiryDay},
		},
		Occurrences: []Occurrence{
			{ID: "o1", LocalDate: "2026-09-26", At: at(t, "2026-09-26 09:00", location), Quantity: quantity(t, "2")},
			{ID: "o2", LocalDate: "2026-09-27", At: at(t, "2026-09-27 09:00", location), Quantity: quantity(t, "2")},
			{ID: "o3", LocalDate: "2026-09-28", At: at(t, "2026-09-28 09:00", location), Quantity: quantity(t, "2")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.CoveredOccurrenceCount != 2 || projection.CoveredPlanDayCount != 2 {
		t.Fatalf("unexpected coverage: %+v", projection)
	}
	if projection.FirstShortfallOccurrence != "o3" || projection.ShortfallQuantity != quantity(t, "1") {
		t.Fatalf("unexpected first shortfall: %+v", projection)
	}
	if projection.StockState != "low" {
		t.Fatalf("stock state = %s, want low", projection.StockState)
	}
	if len(projection.BatchResults) != 3 || projection.BatchResults[0].BatchID != "earlier" ||
		projection.BatchResults[1].BatchID != "later" || projection.BatchResults[2].BatchID != "unknown" {
		t.Fatalf("FEFO order did not keep unknown expiry last: %+v", projection.BatchResults)
	}
}

func TestEvaluateExcludesExpiredInventoryAndKeepsUnknownRisk(t *testing.T) {
	location := time.UTC
	now := at(t, "2026-09-26 08:00", location)
	projection, err := Evaluate(Input{
		Now: now, Location: location, PlanState: "active", PlanValid: true,
		RestockThresholdPlanDays: 0, ExpiryReminderDays: 30,
		Batches: []Batch{
			{ID: "expired", Quantity: quantity(t, "4"), ReceivedAt: now, CreatedAt: now, ExpiryRawValue: "2026-09-25", ExpiryPrecision: ExpiryDay},
			{ID: "unknown", Quantity: quantity(t, "2"), ReceivedAt: now, CreatedAt: now, ExpiryPrecision: ExpiryUnknown},
		},
		Occurrences: []Occurrence{{ID: "o1", At: at(t, "2026-09-26 09:00", location), Quantity: quantity(t, "2")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.OnHandQuantity != quantity(t, "6") || projection.AutoAllocatableQuantity != quantity(t, "2") || projection.ExpiredQuantity != quantity(t, "4") {
		t.Fatalf("unexpected inventory quantities: %+v", projection)
	}
	if projection.ProjectedDepletionAt == nil || projection.RiskState != "expired" {
		t.Fatalf("expected depletion using only allocatable inventory and expired product risk: %+v", projection)
	}
}

func TestEvaluateMonthPrecisionUsesEarlyAndLateBoundaries(t *testing.T) {
	location := time.UTC
	now := at(t, "2027-03-10 08:00", location)
	projection, err := Evaluate(Input{
		Now: now, Location: location, PlanState: "paused", PlanValid: true,
		RestockThresholdPlanDays: 7, ExpiryReminderDays: 30,
		Batches: []Batch{{ID: "month", Quantity: quantity(t, "3"), ReceivedAt: now, CreatedAt: now, ExpiryRawValue: "2027-03", ExpiryPrecision: ExpiryMonth}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := projection.BatchResults[0]
	if projection.CalculationState != "paused" || result.RiskState != "near_expiry" || !result.PrecisionLimited {
		t.Fatalf("unexpected month-precision result: %+v / %+v", projection, result)
	}
	if result.ExpiryPeriodStart != "2027-03-01" || result.ExpiryPeriodEnd != "2027-03-31" {
		t.Fatalf("unexpected month boundaries: %+v", result)
	}

	after := Input{
		Now: at(t, "2027-04-01 08:00", location), Location: location, PlanState: "paused", PlanValid: true,
		RestockThresholdPlanDays: 7, ExpiryReminderDays: 30, Batches: []Batch{{ID: "month", Quantity: quantity(t, "3"), ExpiryRawValue: "2027-03", ExpiryPrecision: ExpiryMonth}},
	}
	expired, err := Evaluate(after)
	if err != nil {
		t.Fatal(err)
	}
	if expired.BatchResults[0].RiskState != "expired" {
		t.Fatalf("month precision expired before/after wrong boundary: %+v", expired.BatchResults[0])
	}
}

func TestEvaluateMarksUnfinishablePerBatch(t *testing.T) {
	location := time.UTC
	now := at(t, "2026-09-26 08:00", location)
	projection, err := Evaluate(Input{
		Now: now, Location: location, PlanState: "active", PlanValid: true,
		RestockThresholdPlanDays: 0, ExpiryReminderDays: 0,
		Batches:     []Batch{{ID: "soon", Quantity: quantity(t, "3"), ReceivedAt: now, CreatedAt: now, ExpiryRawValue: "2026-09-27", ExpiryPrecision: ExpiryDay}},
		Occurrences: []Occurrence{{ID: "o1", At: at(t, "2026-09-26 09:00", location), Quantity: quantity(t, "1")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := projection.BatchResults[0]
	if result.RiskState != "unfinishable" || result.ProjectedRemainingAtExpiry != quantity(t, "2") || projection.RiskState != "unfinishable" {
		t.Fatalf("unexpected unfinishable result: %+v / %+v", projection, result)
	}
}

func TestEvaluateRejectsAmbiguousOrInvalidInputs(t *testing.T) {
	_, err := Evaluate(Input{Now: time.Now(), Location: time.UTC, PlanState: "active", PlanValid: true, Batches: []Batch{{ID: "bad", Quantity: 1, ExpiryRawValue: "03/2027", ExpiryPrecision: ExpiryMonth}}})
	if err == nil {
		t.Fatal("invalid month expiry was accepted")
	}
	_, err = Evaluate(Input{Now: time.Now(), Location: time.UTC, PlanState: "active", PlanValid: true, ExpiryReminderDays: -1})
	if err == nil {
		t.Fatal("negative reminder window was accepted")
	}
}

func TestEvaluateIgnoresPastOccurrencesAndDepletedBatchRisk(t *testing.T) {
	location := time.UTC
	now := at(t, "2026-09-26 08:00", location)
	projection, err := Evaluate(Input{
		Now: now, Location: location, PlanState: "active", PlanValid: true,
		Batches:     []Batch{{ID: "used", Quantity: 0, LifecycleState: "depleted", ExpiryPrecision: ExpiryUnknown}},
		Occurrences: []Occurrence{{ID: "past", At: at(t, "2026-09-25 09:00", location), Quantity: quantity(t, "1")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if projection.CalculationState != "no_future_plan" || projection.StockState != "depleted" || projection.RiskState != "safe" {
		t.Fatalf("past occurrence or depleted batch changed current projection: %+v", projection)
	}
}
