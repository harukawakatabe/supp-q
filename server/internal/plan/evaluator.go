package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	dateLayout = "2006-01-02"
	timeLayout = "15:04"

	TimeResolutionExact          = "exact"
	TimeResolutionDSTGapShifted  = "dst_gap_shifted"
	TimeResolutionDSTFoldEarlier = "dst_fold_earlier"
)

// DayCycle describes a repeating N-day cycle whose first day is AnchorDate.
// A disabled cycle always matches and its remaining fields are ignored.
type DayCycle struct {
	Enabled    bool
	CycleDays  int
	TakeDays   int
	AnchorDate string
}

// LongCycle describes a repeating take/rest cycle in contiguous seven-day
// blocks. A disabled cycle always matches and its remaining fields are ignored.
type LongCycle struct {
	Enabled    bool
	TakeWeeks  int
	RestWeeks  int
	AnchorDate string
}

// DoseSlot is the smallest planned execution unit. Quantity and MealRelation
// are copied into occurrences but are not interpreted by the evaluator.
type DoseSlot struct {
	ID           string
	LocalTime    string
	Quantity     string
	MealRelation string
	Label        string
}

// ScheduleVersion is an immutable schedule snapshot. EffectiveFrom is
// inclusive and EffectiveTo is exclusive. All dates and wall times are
// interpreted in TimeZone.
type ScheduleVersion struct {
	ID            string
	EffectiveFrom string
	EffectiveTo   string
	TimeZone      string
	PlanStartDate string
	Weekdays      []int
	DayCycle      DayCycle
	LongCycle     LongCycle
	DoseSlots     []DoseSlot
}

// Evaluation records each deterministic layer so shadow comparisons can
// explain why a date did or did not produce planned occurrences.
type Evaluation struct {
	LocalDate        string
	VersionApplies   bool
	PlanStarted      bool
	WeeklyMatched    bool
	DayCycleMatched  bool
	LongCycleMatched bool
	IsPlanDay        bool
	Occurrences      []Occurrence
}

// Occurrence is a deterministic projection of one schedule version, local
// date, and dose slot. ScheduledAt is the unique UTC instant selected for the
// local wall time. ResolvedLocalTime differs from LocalTime only for a DST gap.
type Occurrence struct {
	ID                string
	ScheduleVersionID string
	LocalDate         string
	DoseSlotID        string
	LocalTime         string
	ResolvedLocalTime string
	Quantity          string
	MealRelation      string
	Label             string
	TimeZone          string
	ScheduledAt       time.Time
	UTCOffsetMinutes  int
	TimeResolution    string
}

// EvaluateDate evaluates a single local calendar date. Inventory is
// intentionally absent: stock availability cannot change whether a plan day
// or occurrence exists.
func EvaluateDate(version ScheduleVersion, localDate string) (Evaluation, error) {
	result := Evaluation{LocalDate: localDate, Occurrences: []Occurrence{}}

	if version.ID == "" {
		return result, errors.New("schedule version id is required")
	}
	target, err := parseDate("local date", localDate)
	if err != nil {
		return result, err
	}
	effectiveFrom, err := parseDate("effective from", version.EffectiveFrom)
	if err != nil {
		return result, err
	}
	var effectiveTo time.Time
	if version.EffectiveTo != "" {
		effectiveTo, err = parseDate("effective to", version.EffectiveTo)
		if err != nil {
			return result, err
		}
		if !effectiveTo.After(effectiveFrom) {
			return result, errors.New("effective to must be after effective from")
		}
	}
	planStart, err := parseDate("plan start date", version.PlanStartDate)
	if err != nil {
		return result, err
	}
	location, err := time.LoadLocation(version.TimeZone)
	if err != nil {
		return result, fmt.Errorf("load IANA timezone %q: %w", version.TimeZone, err)
	}
	weekdays, err := normalizeWeekdays(version.Weekdays)
	if err != nil {
		return result, err
	}
	if err = validateSlots(version.DoseSlots); err != nil {
		return result, err
	}

	result.VersionApplies = !target.Before(effectiveFrom) && (effectiveTo.IsZero() || target.Before(effectiveTo))
	result.PlanStarted = !target.Before(planStart)
	result.WeeklyMatched = containsWeekday(weekdays, int(target.Weekday()))
	result.DayCycleMatched, err = matchDayCycle(version.DayCycle, target)
	if err != nil {
		return result, err
	}
	result.LongCycleMatched, err = matchLongCycle(version.LongCycle, target)
	if err != nil {
		return result, err
	}
	result.IsPlanDay = result.VersionApplies && result.PlanStarted && result.WeeklyMatched && result.DayCycleMatched && result.LongCycleMatched
	if !result.IsPlanDay {
		return result, nil
	}

	slots := append([]DoseSlot(nil), version.DoseSlots...)
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].LocalTime == slots[j].LocalTime {
			return slots[i].ID < slots[j].ID
		}
		return slots[i].LocalTime < slots[j].LocalTime
	})
	for _, slot := range slots {
		hour, minute, parseErr := parseWallTime(slot.LocalTime)
		if parseErr != nil {
			return result, parseErr
		}
		instant, resolvedWall, offset, resolution, resolveErr := resolveWallTime(target, hour, minute, location)
		if resolveErr != nil {
			return result, fmt.Errorf("resolve dose slot %q: %w", slot.ID, resolveErr)
		}
		result.Occurrences = append(result.Occurrences, Occurrence{
			ID:                occurrenceID(version.ID, localDate, slot.ID),
			ScheduleVersionID: version.ID,
			LocalDate:         localDate,
			DoseSlotID:        slot.ID,
			LocalTime:         slot.LocalTime,
			ResolvedLocalTime: resolvedWall,
			Quantity:          slot.Quantity,
			MealRelation:      slot.MealRelation,
			Label:             slot.Label,
			TimeZone:          version.TimeZone,
			ScheduledAt:       instant.UTC(),
			UTCOffsetMinutes:  offset / 60,
			TimeResolution:    resolution,
		})
	}
	return result, nil
}

func parseDate(name, value string) (time.Time, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD: %w", name, err)
	}
	return parsed.UTC(), nil
}

func normalizeWeekdays(input []int) ([]int, error) {
	if len(input) == 0 {
		return nil, errors.New("at least one weekday is required")
	}
	seen := make(map[int]bool, len(input))
	result := make([]int, 0, len(input))
	for _, weekday := range input {
		if weekday < 0 || weekday > 6 {
			return nil, fmt.Errorf("weekday %d is outside 0..6", weekday)
		}
		if !seen[weekday] {
			seen[weekday] = true
			result = append(result, weekday)
		}
	}
	sort.Ints(result)
	return result, nil
}

func validateSlots(slots []DoseSlot) error {
	if len(slots) == 0 {
		return errors.New("at least one dose slot is required")
	}
	seenIDs := make(map[string]bool, len(slots))
	seenTimes := make(map[string]bool, len(slots))
	for _, slot := range slots {
		if slot.ID == "" {
			return errors.New("dose slot id is required")
		}
		if seenIDs[slot.ID] {
			return fmt.Errorf("duplicate dose slot id %q", slot.ID)
		}
		seenIDs[slot.ID] = true
		if _, _, err := parseWallTime(slot.LocalTime); err != nil {
			return err
		}
		if seenTimes[slot.LocalTime] {
			return fmt.Errorf("duplicate dose slot local time %q", slot.LocalTime)
		}
		seenTimes[slot.LocalTime] = true
	}
	return nil
}

func parseWallTime(value string) (int, int, error) {
	parsed, err := time.Parse(timeLayout, value)
	if err != nil {
		return 0, 0, fmt.Errorf("local time %q must use HH:MM: %w", value, err)
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func matchDayCycle(cycle DayCycle, target time.Time) (bool, error) {
	if !cycle.Enabled {
		return true, nil
	}
	if cycle.CycleDays <= 0 || cycle.TakeDays <= 0 || cycle.TakeDays > cycle.CycleDays {
		return false, errors.New("day cycle requires 0 < take days <= cycle days")
	}
	anchor, err := parseDate("day cycle anchor date", cycle.AnchorDate)
	if err != nil {
		return false, err
	}
	elapsed := calendarDays(anchor, target)
	return elapsed >= 0 && elapsed%cycle.CycleDays < cycle.TakeDays, nil
}

func matchLongCycle(cycle LongCycle, target time.Time) (bool, error) {
	if !cycle.Enabled {
		return true, nil
	}
	if cycle.TakeWeeks <= 0 || cycle.RestWeeks < 0 {
		return false, errors.New("long cycle requires positive take weeks and non-negative rest weeks")
	}
	anchor, err := parseDate("long cycle anchor date", cycle.AnchorDate)
	if err != nil {
		return false, err
	}
	elapsed := calendarDays(anchor, target)
	takeDays := cycle.TakeWeeks * 7
	cycleDays := takeDays + cycle.RestWeeks*7
	return elapsed >= 0 && elapsed%cycleDays < takeDays, nil
}

func calendarDays(from, to time.Time) int {
	return int(to.UTC().Sub(from.UTC()).Hours() / 24)
}

func containsWeekday(weekdays []int, target int) bool {
	index := sort.SearchInts(weekdays, target)
	return index < len(weekdays) && weekdays[index] == target
}

func occurrenceID(scheduleVersionID, localDate, slotID string) string {
	digest := sha256.Sum256([]byte(scheduleVersionID + "|" + localDate + "|" + slotID))
	value := hex.EncodeToString(digest[:])
	// Mirror r1_scheduled_occurrence_id exactly. The SHA-256 digest supplies
	// the payload; the UUID version and variant nibbles make it a valid UUID.
	variant := "89ab"[digest[8]%4]
	return fmt.Sprintf("%s-%s-5%s-%c%s-%s", value[0:8], value[8:12], value[13:16], variant, value[17:20], value[20:32])
}

// resolveWallTime maps a local wall time to exactly one instant. Gaps are
// shifted forward by the size of the offset jump (02:30 -> 03:30 for a
// one-hour gap). Folds choose the earlier UTC instant.
func resolveWallTime(localDate time.Time, hour, minute int, location *time.Location) (time.Time, string, int, string, error) {
	wall := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), hour, minute, 0, 0, time.UTC)
	offsets := offsetsNearWall(wall, location)
	candidates := instantsForWall(wall, location, offsets)
	if len(candidates) > 0 {
		chosen := candidates[0]
		_, offset := chosen.In(location).Zone()
		resolution := TimeResolutionExact
		if len(candidates) > 1 {
			resolution = TimeResolutionDSTFoldEarlier
		}
		return chosen, chosen.In(location).Format(timeLayout), offset, resolution, nil
	}

	before, beforeOK := nearestValidWall(wall, -1, location, offsets)
	after, afterOK := nearestValidWall(wall, 1, location, offsets)
	if !beforeOK || !afterOK {
		return time.Time{}, "", 0, "", errors.New("cannot resolve local wall time around timezone transition")
	}
	_, beforeOffset := before.In(location).Zone()
	_, afterOffset := after.In(location).Zone()
	gapSeconds := afterOffset - beforeOffset
	if gapSeconds <= 0 {
		return time.Time{}, "", 0, "", errors.New("local wall time has no matching instant and is not a forward gap")
	}
	shiftedWall := wall.Add(time.Duration(gapSeconds) * time.Second)
	shiftedCandidates := instantsForWall(shiftedWall, location, offsets)
	if len(shiftedCandidates) == 0 {
		return time.Time{}, "", 0, "", errors.New("shifted local wall time is still invalid")
	}
	chosen := shiftedCandidates[0]
	_, offset := chosen.In(location).Zone()
	return chosen, chosen.In(location).Format(timeLayout), offset, TimeResolutionDSTGapShifted, nil
}

func offsetsNearWall(wall time.Time, location *time.Location) []int {
	seen := map[int]bool{}
	// Seventy-two hours on either side includes both sides of ordinary DST
	// changes and even whole-day civil-time skips.
	for cursor := wall.Add(-72 * time.Hour); !cursor.After(wall.Add(72 * time.Hour)); cursor = cursor.Add(15 * time.Minute) {
		_, offset := cursor.In(location).Zone()
		seen[offset] = true
	}
	result := make([]int, 0, len(seen))
	for offset := range seen {
		result = append(result, offset)
	}
	sort.Ints(result)
	return result
}

func instantsForWall(wall time.Time, location *time.Location, offsets []int) []time.Time {
	result := make([]time.Time, 0, 2)
	seen := map[int64]bool{}
	for _, offset := range offsets {
		candidate := wall.Add(-time.Duration(offset) * time.Second)
		local := candidate.In(location)
		if !sameWall(local, wall) || seen[candidate.UnixNano()] {
			continue
		}
		seen[candidate.UnixNano()] = true
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Before(result[j]) })
	return result
}

func sameWall(local, wall time.Time) bool {
	return local.Year() == wall.Year() && local.Month() == wall.Month() && local.Day() == wall.Day() &&
		local.Hour() == wall.Hour() && local.Minute() == wall.Minute() && local.Second() == wall.Second()
}

func nearestValidWall(wall time.Time, direction int, location *time.Location, offsets []int) (time.Time, bool) {
	for minutes := 1; minutes <= 48*60; minutes++ {
		candidateWall := wall.Add(time.Duration(direction*minutes) * time.Minute)
		candidates := instantsForWall(candidateWall, location, offsets)
		if len(candidates) == 0 {
			continue
		}
		if direction < 0 {
			return candidates[len(candidates)-1], true
		}
		return candidates[0], true
	}
	return time.Time{}, false
}
