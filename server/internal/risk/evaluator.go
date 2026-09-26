package risk

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"suppq.local/server/internal/core"
)

type ExpiryPrecision string

const (
	ExpiryDay     ExpiryPrecision = "day"
	ExpiryMonth   ExpiryPrecision = "month"
	ExpiryYear    ExpiryPrecision = "year"
	ExpiryUnknown ExpiryPrecision = "unknown"
)

type Batch struct {
	ID              string
	Quantity        core.Quantity
	ReceivedAt      time.Time
	CreatedAt       time.Time
	LifecycleState  string
	ExpiryRawValue  string
	ExpiryPrecision ExpiryPrecision
}

type Occurrence struct {
	ID                string
	ScheduleVersionID string
	LocalDate         string
	At                time.Time
	Quantity          core.Quantity
}

type Input struct {
	Now                      time.Time
	Location                 *time.Location
	PlanState                string
	PlanValid                bool
	HorizonReached           bool
	RestockThresholdPlanDays int
	ExpiryReminderDays       int
	Batches                  []Batch
	Occurrences              []Occurrence
}

type BatchResult struct {
	BatchID                    string
	RiskState                  string
	PrecisionLimited           bool
	ExpiryPeriodStart          string
	ExpiryPeriodEnd            string
	ProjectedRemainingAtExpiry core.Quantity
}

type Projection struct {
	CalculationState         string
	StockState               string
	RiskState                string
	OnHandQuantity           core.Quantity
	AutoAllocatableQuantity  core.Quantity
	ExpiredQuantity          core.Quantity
	UnknownExpiryQuantity    core.Quantity
	CoveredOccurrenceCount   int
	CoveredPlanDayCount      int
	ProjectedDepletionAt     *time.Time
	FirstShortfallAt         *time.Time
	FirstShortfallOccurrence string
	ShortfallQuantity        core.Quantity
	BatchResults             []BatchResult
}

type expiryPeriod struct {
	known bool
	start time.Time
	end   time.Time
}

type workingBatch struct {
	batch     Batch
	period    expiryPeriod
	remaining core.Quantity
}

func Evaluate(input Input) (Projection, error) {
	if input.Location == nil {
		return Projection{}, errors.New("location is required")
	}
	if input.Now.IsZero() {
		return Projection{}, errors.New("calculation time is required")
	}
	if input.RestockThresholdPlanDays < 0 || input.ExpiryReminderDays < 0 {
		return Projection{}, errors.New("risk thresholds cannot be negative")
	}
	if input.PlanState != "active" && input.PlanState != "paused" {
		return Projection{}, errors.New("plan state must be active or paused")
	}

	today := localDate(input.Now, input.Location)
	working := make([]workingBatch, 0, len(input.Batches))
	projection := Projection{}
	for _, batch := range input.Batches {
		if batch.ID == "" {
			return Projection{}, errors.New("batch id is required")
		}
		if batch.Quantity < 0 {
			return Projection{}, fmt.Errorf("batch %s has a negative quantity", batch.ID)
		}
		period, err := parseExpiry(batch, input.Location)
		if err != nil {
			return Projection{}, err
		}
		item := workingBatch{batch: batch, period: period, remaining: batch.Quantity}
		working = append(working, item)
		if batch.Quantity == 0 || batch.LifecycleState == "voided" {
			continue
		}
		projection.OnHandQuantity += batch.Quantity
		if !period.known {
			projection.UnknownExpiryQuantity += batch.Quantity
			projection.AutoAllocatableQuantity += batch.Quantity
			continue
		}
		if today.After(period.end) {
			projection.ExpiredQuantity += batch.Quantity
			continue
		}
		projection.AutoAllocatableQuantity += batch.Quantity
	}

	sort.SliceStable(working, func(i, j int) bool {
		left, right := working[i], working[j]
		if left.batch.LifecycleState == "voided" || left.batch.Quantity == 0 {
			return false
		}
		if right.batch.LifecycleState == "voided" || right.batch.Quantity == 0 {
			return true
		}
		if left.period.known != right.period.known {
			return left.period.known
		}
		if left.period.known && !left.period.end.Equal(right.period.end) {
			return left.period.end.Before(right.period.end)
		}
		if !left.batch.ReceivedAt.Equal(right.batch.ReceivedAt) {
			return left.batch.ReceivedAt.Before(right.batch.ReceivedAt)
		}
		if !left.batch.CreatedAt.Equal(right.batch.CreatedAt) {
			return left.batch.CreatedAt.Before(right.batch.CreatedAt)
		}
		return left.batch.ID < right.batch.ID
	})

	occurrences := append([]Occurrence(nil), input.Occurrences...)
	for _, occurrence := range occurrences {
		if occurrence.ID == "" || occurrence.At.IsZero() || occurrence.Quantity <= 0 {
			return Projection{}, errors.New("occurrences require id, instant, and positive quantity")
		}
	}
	sort.SliceStable(occurrences, func(i, j int) bool {
		if occurrences[i].At.Equal(occurrences[j].At) {
			return occurrences[i].ID < occurrences[j].ID
		}
		return occurrences[i].At.Before(occurrences[j].At)
	})

	simulatedQuantity := projection.AutoAllocatableQuantity
	coveredDates := map[string]bool{}
	shortfallSeen := false
	futureOccurrenceCount := 0
	if input.PlanState == "active" && input.PlanValid {
		for _, occurrence := range occurrences {
			if occurrence.At.Before(input.Now) {
				continue
			}
			futureOccurrenceCount++
			dateKey := occurrence.LocalDate
			if dateKey == "" {
				dateKey = occurrence.At.In(input.Location).Format("2006-01-02")
			}
			needed := occurrence.Quantity
			for index := range working {
				candidate := &working[index]
				if candidate.remaining <= 0 || candidate.batch.LifecycleState == "voided" {
					continue
				}
				occurrenceDate := localDate(occurrence.At, input.Location)
				if candidate.period.known && occurrenceDate.After(candidate.period.end) {
					continue
				}
				allocated := candidate.remaining
				if allocated > needed {
					allocated = needed
				}
				candidate.remaining -= allocated
				needed -= allocated
				simulatedQuantity -= allocated
				if needed == 0 {
					break
				}
			}
			if needed > 0 {
				if !shortfallSeen {
					at := occurrence.At
					projection.FirstShortfallAt = &at
					projection.FirstShortfallOccurrence = occurrence.ID
					projection.ShortfallQuantity = needed
				}
				shortfallSeen = true
				coveredDates[dateKey] = false
				break
			}
			projection.CoveredOccurrenceCount++
			if _, exists := coveredDates[dateKey]; !exists {
				coveredDates[dateKey] = true
			}
			if simulatedQuantity == 0 && projection.ProjectedDepletionAt == nil {
				at := occurrence.At
				projection.ProjectedDepletionAt = &at
			}
		}
	}
	for _, complete := range coveredDates {
		if complete {
			projection.CoveredPlanDayCount++
		}
	}

	switch {
	case !input.PlanValid:
		projection.CalculationState = "invalid_plan"
	case input.PlanState == "paused":
		projection.CalculationState = "paused"
	case futureOccurrenceCount == 0:
		projection.CalculationState = "no_future_plan"
	case input.HorizonReached && projection.FirstShortfallAt == nil && simulatedQuantity > 0:
		projection.CalculationState = "beyond_horizon"
	default:
		projection.CalculationState = "calculated"
	}

	switch {
	case projection.AutoAllocatableQuantity == 0:
		projection.StockState = "depleted"
	case input.PlanState == "active" && input.PlanValid && futureOccurrenceCount > 0 &&
		projection.CoveredPlanDayCount <= input.RestockThresholdPlanDays:
		projection.StockState = "low"
	default:
		projection.StockState = "in_stock"
	}

	projection.BatchResults = make([]BatchResult, 0, len(working))
	for _, item := range working {
		result := BatchResult{BatchID: item.batch.ID}
		switch {
		case item.batch.Quantity == 0 || item.batch.LifecycleState == "depleted":
			result.RiskState = "depleted"
		case item.batch.LifecycleState == "voided":
			result.RiskState = "depleted"
		case !item.period.known:
			result.RiskState = "unknown"
		case today.After(item.period.end):
			result.RiskState = "expired"
		case input.PlanState == "active" && input.PlanValid && item.remaining > 0:
			result.RiskState = "unfinishable"
			result.ProjectedRemainingAtExpiry = item.remaining
		default:
			windowStart := item.period.start.AddDate(0, 0, -input.ExpiryReminderDays)
			if !today.Before(windowStart) {
				result.RiskState = "near_expiry"
			} else {
				result.RiskState = "safe"
			}
		}
		if item.period.known {
			result.ExpiryPeriodStart = item.period.start.Format("2006-01-02")
			result.ExpiryPeriodEnd = item.period.end.Format("2006-01-02")
			result.PrecisionLimited = item.batch.ExpiryPrecision == ExpiryMonth || item.batch.ExpiryPrecision == ExpiryYear
		}
		projection.BatchResults = append(projection.BatchResults, result)
	}
	projection.RiskState = aggregateRisk(projection.BatchResults)
	return projection, nil
}

func parseExpiry(batch Batch, location *time.Location) (expiryPeriod, error) {
	raw := strings.TrimSpace(batch.ExpiryRawValue)
	precision := batch.ExpiryPrecision
	if precision == "" && raw == "" {
		precision = ExpiryUnknown
	}
	switch precision {
	case ExpiryUnknown:
		return expiryPeriod{}, nil
	case ExpiryDay:
		value, err := time.ParseInLocation("2006-01-02", raw, location)
		if err != nil {
			return expiryPeriod{}, fmt.Errorf("batch %s has an invalid day expiry", batch.ID)
		}
		return expiryPeriod{known: true, start: value, end: value}, nil
	case ExpiryMonth:
		value, err := time.ParseInLocation("2006-01", raw, location)
		if err != nil {
			return expiryPeriod{}, fmt.Errorf("batch %s has an invalid month expiry", batch.ID)
		}
		return expiryPeriod{known: true, start: value, end: value.AddDate(0, 1, -1)}, nil
	case ExpiryYear:
		value, err := time.ParseInLocation("2006", raw, location)
		if err != nil {
			return expiryPeriod{}, fmt.Errorf("batch %s has an invalid year expiry", batch.ID)
		}
		return expiryPeriod{known: true, start: value, end: value.AddDate(1, 0, -1)}, nil
	default:
		return expiryPeriod{}, fmt.Errorf("batch %s has unsupported expiry precision %q", batch.ID, precision)
	}
}

func localDate(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func aggregateRisk(results []BatchResult) string {
	priority := map[string]int{
		"safe": 1, "depleted": 1, "unknown": 2, "near_expiry": 3,
		"unfinishable": 4, "expired": 5,
	}
	state, score := "safe", 0
	for _, result := range results {
		if result.RiskState == "depleted" {
			continue
		}
		if priority[result.RiskState] > score {
			state, score = result.RiskState, priority[result.RiskState]
		}
	}
	if len(results) == 0 {
		return "unknown"
	}
	return state
}
