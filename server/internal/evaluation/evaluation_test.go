package evaluation

import "testing"

func TestEvaluateRecognitionGate(t *testing.T) {
	roles := []string{"front", "facts", "expiry"}
	samples := make([]Sample, 30)
	for index := range samples {
		samples[index] = Sample{ID: string(rune('a' + index)), Role: roles[index%len(roles)], Expected: map[string]string{"name": "Vitamin C"}, Predicted: map[string]string{"name": " vitamin  c "}, Status: "recognized", Confidence: .9, LatencyMS: int64(1000 + index)}
	}
	report, err := Evaluate(Dataset{Name: "accepted-private-set", Samples: samples}, ProductionThresholds, false)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.FieldAccuracy != 1 || report.P95LatencyMS != 1028 {
		t.Fatalf("unexpected passing report: %+v", report)
	}
	samples[0].Predicted["name"] = "wrong"
	samples[0].Confidence = .95
	samples[1].Predicted["name"] = "also wrong"
	samples[1].Confidence = .95
	report, err = Evaluate(Dataset{Name: "failing-private-set", Samples: samples}, ProductionThresholds, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || report.FalseConfidenceRate == 0 {
		t.Fatalf("false confidence was not detected: %+v", report)
	}
}

func TestProductionGateRejectsSmallDataset(t *testing.T) {
	_, err := Evaluate(Dataset{Samples: []Sample{{ID: "one", Role: "front", Expected: map[string]string{"name": "x"}}}}, ProductionThresholds, false)
	if err == nil {
		t.Fatal("small dataset must not satisfy production evaluation")
	}
}
