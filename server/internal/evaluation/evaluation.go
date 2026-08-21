package evaluation

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type Sample struct {
	ID              string            `json:"id"`
	Role            string            `json:"role"`
	Expected        map[string]string `json:"expected"`
	Predicted       map[string]string `json:"predicted"`
	Status          string            `json:"status"`
	Confidence      float64           `json:"confidence"`
	CorrectedFields []string          `json:"correctedFields"`
	LatencyMS       int64             `json:"latencyMs"`
	ProviderError   bool              `json:"providerError"`
}

type Dataset struct {
	Name    string   `json:"name"`
	Samples []Sample `json:"samples"`
}

type Thresholds struct {
	FieldAccuracyMin    float64 `json:"fieldAccuracyMin"`
	CorrectionRateMax   float64 `json:"correctionRateMax"`
	UnrecognizedRateMax float64 `json:"unrecognizedRateMax"`
	FalseConfidenceMax  float64 `json:"falseConfidenceRateMax"`
	ProviderFailureMax  float64 `json:"providerFailureRateMax"`
	P95LatencyMSMax     int64   `json:"p95LatencyMsMax"`
}

var ProductionThresholds = Thresholds{FieldAccuracyMin: 0.90, CorrectionRateMax: 0.25, UnrecognizedRateMax: 0.15, FalseConfidenceMax: 0.05, ProviderFailureMax: 0.05, P95LatencyMSMax: 45000}

type Report struct {
	DatasetName         string         `json:"datasetName"`
	SampleCount         int            `json:"sampleCount"`
	RoleCounts          map[string]int `json:"roleCounts"`
	FieldCount          int            `json:"fieldCount"`
	FieldAccuracy       float64        `json:"fieldAccuracy"`
	CorrectionRate      float64        `json:"correctionRate"`
	UnrecognizedRate    float64        `json:"unrecognizedRate"`
	FalseConfidenceRate float64        `json:"falseConfidenceRate"`
	ProviderFailureRate float64        `json:"providerFailureRate"`
	P50LatencyMS        int64          `json:"p50LatencyMs"`
	P95LatencyMS        int64          `json:"p95LatencyMs"`
	Thresholds          Thresholds     `json:"thresholds"`
	Passed              bool           `json:"passed"`
	Failures            []string       `json:"failures"`
}

func normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func percentile(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := int(math.Ceil(percentile*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	return values[index]
}

func Evaluate(dataset Dataset, thresholds Thresholds, allowSmall bool) (Report, error) {
	count := len(dataset.Samples)
	if !allowSmall && (count < 30 || count > 50) {
		return Report{}, fmt.Errorf("production evaluation requires 30-50 images, got %d", count)
	}
	if count == 0 {
		return Report{}, fmt.Errorf("evaluation dataset is empty")
	}
	report := Report{DatasetName: dataset.Name, SampleCount: count, RoleCounts: map[string]int{"front": 0, "facts": 0, "expiry": 0}, Thresholds: thresholds, Failures: []string{}}
	correctFields, correctedImages, unrecognized, falseConfident, providerFailures := 0, 0, 0, 0, 0
	latencies := make([]int64, 0, count)
	for _, sample := range dataset.Samples {
		if sample.ID == "" || (sample.Role != "front" && sample.Role != "facts" && sample.Role != "expiry") {
			return Report{}, fmt.Errorf("sample %q has an invalid id or role", sample.ID)
		}
		report.RoleCounts[sample.Role]++
		if len(sample.CorrectedFields) > 0 {
			correctedImages++
		}
		if sample.Status == "unrecognized" {
			unrecognized++
		}
		if sample.ProviderError {
			providerFailures++
		}
		if sample.LatencyMS >= 0 {
			latencies = append(latencies, sample.LatencyMS)
		}
		mismatch := false
		for field, expected := range sample.Expected {
			report.FieldCount++
			if normalize(sample.Predicted[field]) == normalize(expected) {
				correctFields++
			} else {
				mismatch = true
			}
		}
		if sample.Confidence >= 0.80 && mismatch {
			falseConfident++
		}
	}
	if report.FieldCount == 0 {
		return Report{}, fmt.Errorf("evaluation dataset has no expected fields")
	}
	for role, roleCount := range report.RoleCounts {
		if roleCount == 0 {
			return Report{}, fmt.Errorf("evaluation dataset has no %s images", role)
		}
	}
	report.FieldAccuracy = float64(correctFields) / float64(report.FieldCount)
	report.CorrectionRate = float64(correctedImages) / float64(count)
	report.UnrecognizedRate = float64(unrecognized) / float64(count)
	report.FalseConfidenceRate = float64(falseConfident) / float64(count)
	report.ProviderFailureRate = float64(providerFailures) / float64(count)
	report.P50LatencyMS = percentile(append([]int64(nil), latencies...), .50)
	report.P95LatencyMS = percentile(append([]int64(nil), latencies...), .95)
	if report.FieldAccuracy < thresholds.FieldAccuracyMin {
		report.Failures = append(report.Failures, "field_accuracy")
	}
	if report.CorrectionRate > thresholds.CorrectionRateMax {
		report.Failures = append(report.Failures, "correction_rate")
	}
	if report.UnrecognizedRate > thresholds.UnrecognizedRateMax {
		report.Failures = append(report.Failures, "unrecognized_rate")
	}
	if report.FalseConfidenceRate > thresholds.FalseConfidenceMax {
		report.Failures = append(report.Failures, "false_confidence_rate")
	}
	if report.ProviderFailureRate > thresholds.ProviderFailureMax {
		report.Failures = append(report.Failures, "provider_failure_rate")
	}
	if report.P95LatencyMS > thresholds.P95LatencyMSMax {
		report.Failures = append(report.Failures, "p95_latency")
	}
	report.Passed = len(report.Failures) == 0
	return report, nil
}
