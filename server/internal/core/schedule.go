package core

import (
	"errors"
	"sort"
	"time"
)

const DateLayout = "2006-01-02"

type DayCycleRule struct {
	Enabled       bool      `json:"enabled"`
	CycleDays     int       `json:"cycleDays"`
	TakeDays      int       `json:"takeDays"`
	AnchorDate    time.Time `json:"-"`
	EffectiveDate time.Time `json:"-"`
}

type DayCycle struct {
	Enabled    bool           `json:"enabled"`
	CycleDays  int            `json:"cycleDays"`
	TakeDays   int            `json:"takeDays"`
	AnchorDate time.Time      `json:"-"`
	History    []DayCycleRule `json:"-"`
}

type LongCycle struct {
	Enabled   bool      `json:"enabled"`
	TakeWeeks int       `json:"takeWeeks"`
	RestWeeks int       `json:"restWeeks"`
	StartDate time.Time `json:"-"`
}

type Schedule struct {
	StartDate     time.Time
	Weekdays      []int
	DayCycle      DayCycle
	LongCycle     LongCycle
	ReminderTimes []string
}

type ScheduleMatch struct {
	Started   bool `json:"started"`
	Weekly    bool `json:"weekly"`
	DayCycle  bool `json:"dayCycle"`
	LongCycle bool `json:"longCycle"`
	Active    bool `json:"active"`
}

func ParseDate(value string) (time.Time, error) {
	date, err := time.Parse(DateLayout, value)
	if err != nil {
		return time.Time{}, errors.New("date must use YYYY-MM-DD")
	}
	return date.UTC(), nil
}

func DateKey(value time.Time) string              { return value.UTC().Format(DateLayout) }
func AddDays(value time.Time, days int) time.Time { return value.UTC().AddDate(0, 0, days) }
func DiffDays(from, to time.Time) int             { return int(to.UTC().Sub(from.UTC()).Hours() / 24) }

func NormalizeSchedule(schedule Schedule) (Schedule, error) {
	if schedule.StartDate.IsZero() {
		return Schedule{}, errors.New("schedule start date is required")
	}
	seen := map[int]bool{}
	weekdays := make([]int, 0, len(schedule.Weekdays))
	for _, day := range schedule.Weekdays {
		if day < 0 || day > 6 || seen[day] {
			continue
		}
		seen[day] = true
		weekdays = append(weekdays, day)
	}
	if len(weekdays) == 0 {
		return Schedule{}, errors.New("at least one weekday is required")
	}
	sort.Ints(weekdays)
	schedule.Weekdays = weekdays
	if schedule.DayCycle.CycleDays <= 0 {
		schedule.DayCycle.CycleDays = 28
	}
	if schedule.DayCycle.TakeDays <= 0 {
		schedule.DayCycle.TakeDays = schedule.DayCycle.CycleDays
	}
	if schedule.DayCycle.TakeDays > schedule.DayCycle.CycleDays {
		return Schedule{}, errors.New("day-cycle take days cannot exceed cycle days")
	}
	if schedule.DayCycle.AnchorDate.IsZero() {
		schedule.DayCycle.AnchorDate = schedule.StartDate
	}
	if schedule.LongCycle.TakeWeeks <= 0 {
		schedule.LongCycle.TakeWeeks = 6
	}
	if schedule.LongCycle.RestWeeks < 0 {
		return Schedule{}, errors.New("long-cycle rest weeks cannot be negative")
	}
	if schedule.LongCycle.StartDate.IsZero() {
		schedule.LongCycle.StartDate = schedule.StartDate
	}
	if len(schedule.ReminderTimes) == 0 {
		schedule.ReminderTimes = []string{"09:00"}
	}
	if len(schedule.DayCycle.History) == 0 {
		schedule.DayCycle.History = []DayCycleRule{{Enabled: schedule.DayCycle.Enabled, CycleDays: schedule.DayCycle.CycleDays, TakeDays: schedule.DayCycle.TakeDays, AnchorDate: schedule.DayCycle.AnchorDate, EffectiveDate: schedule.DayCycle.AnchorDate}}
	}
	for index := range schedule.DayCycle.History {
		rule := &schedule.DayCycle.History[index]
		if rule.CycleDays <= 0 {
			rule.CycleDays = schedule.DayCycle.CycleDays
		}
		if rule.TakeDays <= 0 {
			rule.TakeDays = schedule.DayCycle.TakeDays
		}
		if rule.TakeDays > rule.CycleDays {
			return Schedule{}, errors.New("history take days cannot exceed cycle days")
		}
		if rule.AnchorDate.IsZero() {
			rule.AnchorDate = schedule.DayCycle.AnchorDate
		}
		if rule.EffectiveDate.IsZero() {
			rule.EffectiveDate = rule.AnchorDate
		}
	}
	sort.Slice(schedule.DayCycle.History, func(i, j int) bool {
		return schedule.DayCycle.History[i].EffectiveDate.Before(schedule.DayCycle.History[j].EffectiveDate)
	})
	return schedule, nil
}

func EffectiveStart(schedule Schedule) time.Time {
	start := schedule.StartDate
	if schedule.DayCycle.Enabled && schedule.DayCycle.AnchorDate.Before(start) {
		start = schedule.DayCycle.AnchorDate
	}
	if schedule.LongCycle.Enabled && schedule.LongCycle.StartDate.Before(start) {
		start = schedule.LongCycle.StartDate
	}
	return start
}

func dayRuleOn(schedule Schedule, target time.Time) DayCycleRule {
	history := schedule.DayCycle.History
	selected := DayCycleRule{Enabled: schedule.DayCycle.Enabled, CycleDays: schedule.DayCycle.CycleDays, TakeDays: schedule.DayCycle.TakeDays, AnchorDate: schedule.DayCycle.AnchorDate, EffectiveDate: schedule.DayCycle.AnchorDate}
	for _, rule := range history {
		if rule.EffectiveDate.After(target) {
			break
		}
		selected = rule
	}
	return selected
}

func MatchSchedule(schedule Schedule, status string, inventory Quantity, target time.Time) ScheduleMatch {
	if status != "active" || inventory <= 0 {
		return ScheduleMatch{}
	}
	normalized, err := NormalizeSchedule(schedule)
	if err != nil {
		return ScheduleMatch{}
	}
	target = target.UTC()
	started := !target.Before(EffectiveStart(normalized))
	weekly := started && containsDay(normalized.Weekdays, int(target.Weekday()))
	rule := dayRuleOn(normalized, target)
	dayElapsed := DiffDays(rule.AnchorDate, target)
	dayMatch := started && (!rule.Enabled || (dayElapsed >= 0 && dayElapsed%rule.CycleDays < rule.TakeDays))
	longElapsed := DiffDays(normalized.LongCycle.StartDate, target)
	takeDays := normalized.LongCycle.TakeWeeks * 7
	fullDays := takeDays + normalized.LongCycle.RestWeeks*7
	longMatch := started && (!normalized.LongCycle.Enabled || (longElapsed >= 0 && longElapsed%fullDays < takeDays))
	return ScheduleMatch{Started: started, Weekly: weekly, DayCycle: dayMatch, LongCycle: longMatch, Active: started && weekly && dayMatch && longMatch}
}

func containsDay(days []int, target int) bool {
	for _, day := range days {
		if day == target {
			return true
		}
	}
	return false
}

func ProjectedFinish(schedule Schedule, status string, inventory, daily Quantity, from time.Time) *time.Time {
	remaining := CeilDiv(inventory, daily)
	if remaining <= 0 {
		value := from.UTC()
		return &value
	}
	cursor := from.UTC()
	for count := 0; count < 36525; count++ {
		if MatchSchedule(schedule, status, inventory, cursor).Active {
			remaining--
			if remaining == 0 {
				result := cursor
				return &result
			}
		}
		cursor = AddDays(cursor, 1)
	}
	return nil
}

func LatestStart(schedule Schedule, status string, inventory, daily Quantity, expiry time.Time) *time.Time {
	remaining := CeilDiv(inventory, daily)
	if remaining <= 0 {
		result := expiry.UTC()
		return &result
	}
	cursor := expiry.UTC()
	for count := 0; count < 36525; count++ {
		if MatchSchedule(schedule, status, inventory, cursor).Active {
			remaining--
			if remaining == 0 {
				result := cursor
				return &result
			}
		}
		cursor = AddDays(cursor, -1)
	}
	return nil
}

type ExpiryRisk struct {
	Level               string `json:"level"`
	Message             string `json:"message"`
	ExpiryDate          string `json:"expiryDate,omitempty"`
	ProjectedFinishDate string `json:"projectedFinishDate,omitempty"`
	LatestStartDate     string `json:"latestStartDate,omitempty"`
	DaysToExpiry        int    `json:"daysToExpiry,omitempty"`
}

func CalculateExpiryRisk(schedule Schedule, status string, inventory, daily Quantity, expiry *time.Time, reminderDays int, from time.Time) ExpiryRisk {
	if expiry == nil {
		return ExpiryRisk{Level: "none", Message: "未设置有效期"}
	}
	finish := ProjectedFinish(schedule, status, inventory, daily, from)
	latest := LatestStart(schedule, status, inventory, daily, *expiry)
	days := DiffDays(from, *expiry)
	risk := ExpiryRisk{ExpiryDate: DateKey(*expiry), DaysToExpiry: days}
	if finish != nil {
		risk.ProjectedFinishDate = DateKey(*finish)
	}
	if latest != nil {
		risk.LatestStartDate = DateKey(*latest)
	}
	if days < 0 {
		risk.Level = "danger"
		risk.Message = "已过期"
		return risk
	}
	if finish != nil && finish.After(*expiry) {
		risk.Level = "danger"
		risk.Message = "预计无法在有效期前用完"
		return risk
	}
	if days <= reminderDays {
		risk.Level = "warn"
		risk.Message = "接近有效期"
		return risk
	}
	risk.Level = "safe"
	risk.Message = "可按当前计划在有效期前用完"
	return risk
}
