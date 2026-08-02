package core

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	result, err := ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func baseSchedule(t *testing.T) Schedule {
	start := mustDate(t, "2026-07-01")
	schedule, err := NormalizeSchedule(Schedule{StartDate: start, Weekdays: []int{1, 2, 3, 4, 5}, DayCycle: DayCycle{Enabled: false, CycleDays: 28, TakeDays: 28, AnchorDate: start}, LongCycle: LongCycle{Enabled: false, TakeWeeks: 6, RestWeeks: 4, StartDate: start}})
	if err != nil {
		t.Fatal(err)
	}
	return schedule
}

func TestThreeScheduleLayersAreIntersected(t *testing.T) {
	schedule := baseSchedule(t)
	schedule.DayCycle.Enabled = true
	schedule.DayCycle.CycleDays = 4
	schedule.DayCycle.TakeDays = 2
	schedule.DayCycle.History = []DayCycleRule{{Enabled: true, CycleDays: 4, TakeDays: 2, AnchorDate: mustDate(t, "2026-07-01"), EffectiveDate: mustDate(t, "2026-07-01")}}
	schedule.LongCycle = LongCycle{Enabled: true, TakeWeeks: 1, RestWeeks: 1, StartDate: mustDate(t, "2026-07-01")}
	quantity, _ := QuantityFromFloat(10)
	cases := []struct {
		date   string
		active bool
		layer  string
	}{{"2026-07-01", true, "all"}, {"2026-07-03", false, "day"}, {"2026-07-08", false, "long"}, {"2026-07-04", false, "weekly"}}
	for _, testCase := range cases {
		match := MatchSchedule(schedule, "active", quantity, mustDate(t, testCase.date))
		if match.Active != testCase.active {
			t.Fatalf("%s expected %v, got %+v", testCase.date, testCase.active, match)
		}
	}
}

func TestProjectedFinishSkipsRestDays(t *testing.T) {
	schedule := baseSchedule(t)
	inventory, _ := QuantityFromFloat(5)
	daily, _ := QuantityFromFloat(1)
	finish := ProjectedFinish(schedule, "active", inventory, daily, mustDate(t, "2026-07-17"))
	if finish == nil || DateKey(*finish) != "2026-07-23" {
		t.Fatalf("unexpected finish: %v", finish)
	}
}

func TestIndependentAnchorsAndHistoricalRules(t *testing.T) {
	schedule := baseSchedule(t)
	schedule.Weekdays = []int{0, 1, 2, 3, 4, 5, 6}
	schedule.StartDate = mustDate(t, "2026-06-01")
	schedule.LongCycle = LongCycle{Enabled: true, TakeWeeks: 2, RestWeeks: 1, StartDate: mustDate(t, "2026-06-15")}
	quantity, _ := QuantityFromFloat(10)
	if MatchSchedule(schedule, "active", quantity, mustDate(t, "2026-06-14")).Active {
		t.Fatal("long cycle must not activate before its independent anchor")
	}
	if !MatchSchedule(schedule, "active", quantity, mustDate(t, "2026-06-15")).Active {
		t.Fatal("long cycle should activate on its anchor")
	}

	schedule.DayCycle = DayCycle{Enabled: true, CycleDays: 4, TakeDays: 2, AnchorDate: mustDate(t, "2026-06-01"), History: []DayCycleRule{
		{Enabled: true, CycleDays: 4, TakeDays: 2, AnchorDate: mustDate(t, "2026-06-01"), EffectiveDate: mustDate(t, "2026-06-01")},
		{Enabled: true, CycleDays: 6, TakeDays: 6, AnchorDate: mustDate(t, "2026-07-01"), EffectiveDate: mustDate(t, "2026-07-01")},
	}}
	if MatchSchedule(schedule, "active", quantity, mustDate(t, "2026-06-03")).DayCycle {
		t.Fatal("future correction must not change the historical rest day")
	}
	if !MatchSchedule(schedule, "active", quantity, mustDate(t, "2026-07-03")).DayCycle {
		t.Fatal("new day-cycle version should apply from its effective date")
	}
}

func TestQuantityUsesSixDecimalFixedPoint(t *testing.T) {
	left, _ := QuantityFromFloat(0.1)
	right, _ := QuantityFromFloat(0.2)
	if left+right != Quantity(300000) {
		t.Fatalf("unexpected fixed point sum: %d", left+right)
	}
	if (left + right).DatabaseString() != "0.3" {
		t.Fatalf("unexpected database quantity: %s", (left + right).DatabaseString())
	}
}
