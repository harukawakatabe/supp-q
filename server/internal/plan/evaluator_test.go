package plan

import (
	"regexp"
	"testing"
	"time"
)

func baseVersion() ScheduleVersion {
	return ScheduleVersion{
		ID:            "schedule-v1",
		EffectiveFrom: "2026-01-01",
		TimeZone:      "UTC",
		PlanStartDate: "2026-01-01",
		Weekdays:      []int{0, 1, 2, 3, 4, 5, 6},
		DoseSlots:     []DoseSlot{{ID: "morning", LocalTime: "09:00", Quantity: "1", MealRelation: "unspecified"}},
	}
}

func TestEvaluateDateUsesVersionIntervalAndPlanStartHardBoundary(t *testing.T) {
	version := baseVersion()
	version.EffectiveFrom = "2026-12-20"
	version.EffectiveTo = "2027-01-10"
	version.PlanStartDate = "2026-12-28"
	version.DayCycle = DayCycle{Enabled: true, CycleDays: 4, TakeDays: 2, AnchorDate: "2026-12-01"}
	version.LongCycle = LongCycle{Enabled: false}

	beforeStart, err := EvaluateDate(version, "2026-12-25")
	if err != nil {
		t.Fatal(err)
	}
	if !beforeStart.VersionApplies || beforeStart.PlanStarted || beforeStart.IsPlanDay || len(beforeStart.Occurrences) != 0 {
		t.Fatalf("anchor before plan start bypassed hard boundary: %+v", beforeStart)
	}

	crossYear, err := EvaluateDate(version, "2027-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if !crossYear.IsPlanDay || len(crossYear.Occurrences) != 1 {
		t.Fatalf("calendar-day cycle did not cross year correctly: %+v", crossYear)
	}

	atExclusiveEnd, err := EvaluateDate(version, "2027-01-10")
	if err != nil {
		t.Fatal(err)
	}
	if atExclusiveEnd.VersionApplies || atExclusiveEnd.IsPlanDay {
		t.Fatalf("effectiveTo must be exclusive: %+v", atExclusiveEnd)
	}
}

func TestEvaluateDateIntersectsWeeklyDayAndLongCycles(t *testing.T) {
	version := baseVersion()
	version.Weekdays = []int{1}
	version.DayCycle = DayCycle{Enabled: true, CycleDays: 4, TakeDays: 2, AnchorDate: "2026-01-01"}
	version.LongCycle = LongCycle{Enabled: true, TakeWeeks: 1, RestWeeks: 1, AnchorDate: "2026-01-01"}

	active, err := EvaluateDate(version, "2026-01-05")
	if err != nil {
		t.Fatal(err)
	}
	if !active.WeeklyMatched || !active.DayCycleMatched || !active.LongCycleMatched || !active.IsPlanDay {
		t.Fatalf("expected all three layers to match: %+v", active)
	}

	weeklyMiss, err := EvaluateDate(version, "2026-01-06")
	if err != nil {
		t.Fatal(err)
	}
	if weeklyMiss.WeeklyMatched || !weeklyMiss.DayCycleMatched || !weeklyMiss.LongCycleMatched || weeklyMiss.IsPlanDay {
		t.Fatalf("weekly miss did not suppress the AND result: %+v", weeklyMiss)
	}

	cycleMiss, err := EvaluateDate(version, "2026-01-12")
	if err != nil {
		t.Fatal(err)
	}
	if !cycleMiss.WeeklyMatched || cycleMiss.DayCycleMatched || cycleMiss.LongCycleMatched || cycleMiss.IsPlanDay {
		t.Fatalf("cycle misses did not suppress the AND result: %+v", cycleMiss)
	}
}

func TestDisabledCyclesAlwaysMatch(t *testing.T) {
	version := baseVersion()
	version.DayCycle = DayCycle{Enabled: false}
	version.LongCycle = LongCycle{Enabled: false}

	result, err := EvaluateDate(version, "2026-06-15")
	if err != nil {
		t.Fatal(err)
	}
	if !result.DayCycleMatched || !result.LongCycleMatched || !result.IsPlanDay {
		t.Fatalf("disabled cycles must not restrict a plan: %+v", result)
	}
}

func TestEvaluateDateCreatesStableOccurrencePerDoseSlot(t *testing.T) {
	version := baseVersion()
	version.DoseSlots = []DoseSlot{
		{ID: "evening", LocalTime: "20:00", Quantity: "2", MealRelation: "after_meal"},
		{ID: "morning", LocalTime: "08:00", Quantity: "1", MealRelation: "with_meal"},
	}

	first, err := EvaluateDate(version, "2026-02-02")
	if err != nil {
		t.Fatal(err)
	}
	second, err := EvaluateDate(version, "2026-02-02")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Occurrences) != 2 || len(second.Occurrences) != 2 {
		t.Fatalf("expected one occurrence per dose slot: %+v", first.Occurrences)
	}
	if first.Occurrences[0].DoseSlotID != "morning" || first.Occurrences[1].DoseSlotID != "evening" {
		t.Fatalf("occurrences are not ordered by local time: %+v", first.Occurrences)
	}
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for index := range first.Occurrences {
		if first.Occurrences[index].ID != second.Occurrences[index].ID {
			t.Fatalf("repeated evaluation changed occurrence id: %q != %q", first.Occurrences[index].ID, second.Occurrences[index].ID)
		}
		if !uuidPattern.MatchString(first.Occurrences[index].ID) {
			t.Fatalf("occurrence id is not a deterministic UUIDv5-shaped value: %q", first.Occurrences[index].ID)
		}
	}
	if first.Occurrences[0].ID == first.Occurrences[1].ID {
		t.Fatal("different dose slots received the same occurrence id")
	}
	if first.Occurrences[0].ID != "e21a6ba6-e2d3-50b1-bbb8-0f05f2a75618" {
		t.Fatalf("occurrence id diverged from the migration contract: %q", first.Occurrences[0].ID)
	}
}

func TestEvaluateDateResolvesOrdinaryIANAZones(t *testing.T) {
	tests := []struct {
		name       string
		zone       string
		localDate  string
		localTime  string
		wantUTC    string
		wantOffset int
	}{
		{name: "UTC", zone: "UTC", localDate: "2026-02-02", localTime: "09:15", wantUTC: "2026-02-02T09:15:00Z", wantOffset: 0},
		{name: "Asia Shanghai", zone: "Asia/Shanghai", localDate: "2026-02-02", localTime: "09:15", wantUTC: "2026-02-02T01:15:00Z", wantOffset: 8 * 60 * 60},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version := baseVersion()
			version.TimeZone = test.zone
			version.DoseSlots = []DoseSlot{{ID: "slot", LocalTime: test.localTime, Quantity: "1"}}
			result, err := EvaluateDate(version, test.localDate)
			if err != nil {
				t.Fatal(err)
			}
			occurrence := result.Occurrences[0]
			if occurrence.ScheduledAt.Format(time.RFC3339) != test.wantUTC || occurrence.UTCOffsetMinutes != test.wantOffset/60 || occurrence.TimeResolution != TimeResolutionExact {
				t.Fatalf("unexpected timezone resolution: %+v", occurrence)
			}
		})
	}
}

func TestEvaluateDateShiftsNewYorkDSTGapForward(t *testing.T) {
	version := baseVersion()
	version.TimeZone = "America/New_York"
	version.DoseSlots = []DoseSlot{{ID: "gap", LocalTime: "02:30", Quantity: "1"}}

	result, err := EvaluateDate(version, "2026-03-08")
	if err != nil {
		t.Fatal(err)
	}
	occurrence := result.Occurrences[0]
	if occurrence.ScheduledAt.Format(time.RFC3339) != "2026-03-08T07:30:00Z" || occurrence.ResolvedLocalTime != "03:30" || occurrence.UTCOffsetMinutes != -4*60 || occurrence.TimeResolution != TimeResolutionDSTGapShifted {
		t.Fatalf("DST gap was not shifted forward by the gap: %+v", occurrence)
	}
}

func TestEvaluateDateChoosesEarlierNewYorkDSTFoldInstant(t *testing.T) {
	version := baseVersion()
	version.TimeZone = "America/New_York"
	version.DoseSlots = []DoseSlot{{ID: "fold", LocalTime: "01:30", Quantity: "1"}}

	result, err := EvaluateDate(version, "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	occurrence := result.Occurrences[0]
	if occurrence.ScheduledAt.Format(time.RFC3339) != "2026-11-01T05:30:00Z" || occurrence.ResolvedLocalTime != "01:30" || occurrence.UTCOffsetMinutes != -4*60 || occurrence.TimeResolution != TimeResolutionDSTFoldEarlier {
		t.Fatalf("DST fold did not select the earlier instant: %+v", occurrence)
	}
}
