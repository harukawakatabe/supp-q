package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"suppq.local/server/internal/core"
)

var reminderPattern = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

type Error struct{ Code, Message string }

func (value *Error) Error() string         { return value.Code }
func NewError(code, message string) *Error { return &Error{Code: code, Message: message} }

var (
	ErrNotFound              = NewError("resource_not_found", "记录不存在。")
	ErrInsufficientInventory = NewError("insufficient_inventory", "库存不足，未记录本次服用。")
	ErrAlreadyUndone         = NewError("intake_already_undone", "该服用记录已经撤销。")
)

type Scope struct{ UserID, WorkspaceID string }
type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, now: func() time.Time { return time.Now().UTC() }}
}

type IngredientInput struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}
type BatchInput struct {
	Quantity   float64 `json:"quantity"`
	ExpiryDate string  `json:"expiryDate"`
	PriceCNY   float64 `json:"priceCny"`
}
type DayCycleInput struct {
	Enabled    bool   `json:"enabled"`
	CycleDays  int    `json:"cycleDays"`
	TakeDays   int    `json:"takeDays"`
	AnchorDate string `json:"anchorDate"`
}
type LongCycleInput struct {
	Enabled   bool   `json:"enabled"`
	TakeWeeks int    `json:"takeWeeks"`
	RestWeeks int    `json:"restWeeks"`
	StartDate string `json:"startDate"`
}
type ScheduleInput struct {
	StartDate     string         `json:"startDate"`
	Weekdays      []int          `json:"weekdays"`
	DayCycle      DayCycleInput  `json:"dayCycle"`
	LongCycle     LongCycleInput `json:"longCycle"`
	ReminderTimes []string       `json:"reminderTimes"`
}
type CreateProductInput struct {
	Name                      string            `json:"name"`
	Brand                     string            `json:"brand"`
	ProductType               string            `json:"productType"`
	Unit                      string            `json:"unit"`
	DoseQuantity              float64           `json:"doseQuantity"`
	DoseTimesPerDay           int               `json:"doseTimesPerDay"`
	IngredientServingQuantity float64           `json:"ingredientServingQuantity"`
	WithFood                  *bool             `json:"withFood"`
	RestockThresholdDays      int               `json:"restockThresholdDays"`
	ExpiryReminderDays        int               `json:"expiryReminderDays"`
	Schedule                  ScheduleInput     `json:"schedule"`
	OpeningBatch              BatchInput        `json:"openingBatch"`
	Ingredients               []IngredientInput `json:"ingredients"`
}

type ScheduleView struct {
	Version       int            `json:"version"`
	StartDate     string         `json:"startDate"`
	Weekdays      []int          `json:"weekdays"`
	DayCycle      DayCycleInput  `json:"dayCycle"`
	LongCycle     LongCycleInput `json:"longCycle"`
	ReminderTimes []string       `json:"reminderTimes"`
}
type BatchView struct {
	ID              string    `json:"id"`
	InitialQuantity float64   `json:"initialQuantity"`
	CurrentQuantity float64   `json:"currentQuantity"`
	ExpiryDate      string    `json:"expiryDate,omitempty"`
	PriceCNY        float64   `json:"priceCny"`
	CreatedAt       time.Time `json:"createdAt"`
}
type IngredientView struct {
	ID     string  `json:"id"`
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}
type Product struct {
	ID                        string           `json:"id"`
	Name                      string           `json:"name"`
	Brand                     string           `json:"brand"`
	ProductType               string           `json:"productType"`
	Status                    string           `json:"status"`
	Unit                      string           `json:"unit"`
	DoseQuantity              float64          `json:"doseQuantity"`
	DoseTimesPerDay           int              `json:"doseTimesPerDay"`
	DailyQuantity             float64          `json:"dailyQuantity"`
	IngredientServingQuantity float64          `json:"ingredientServingQuantity"`
	WithFood                  *bool            `json:"withFood,omitempty"`
	RestockThresholdDays      int              `json:"restockThresholdDays"`
	ExpiryReminderDays        int              `json:"expiryReminderDays"`
	CurrentQuantity           float64          `json:"currentQuantity"`
	Schedule                  ScheduleView     `json:"schedule"`
	Batches                   []BatchView      `json:"batches"`
	Ingredients               []IngredientView `json:"ingredients"`
	ExpiryRisk                core.ExpiryRisk  `json:"expiryRisk"`
	CreatedAt                 time.Time        `json:"createdAt"`
	UpdatedAt                 time.Time        `json:"updatedAt"`
}

type Intake struct {
	ID          string           `json:"id"`
	ProductID   string           `json:"productId"`
	Date        string           `json:"date"`
	Time        string           `json:"time,omitempty"`
	Quantity    float64          `json:"quantity"`
	Source      string           `json:"source"`
	Status      string           `json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	RevokedAt   *time.Time       `json:"revokedAt,omitempty"`
	Allocations []AllocationView `json:"allocations"`
}
type AllocationView struct {
	BatchID     string  `json:"batchId"`
	Quantity    float64 `json:"quantity"`
	UnitCostCNY float64 `json:"unitCostCny"`
}
type CreateIntakeInput struct {
	ProductID string  `json:"productId"`
	Date      string  `json:"date"`
	Time      string  `json:"time"`
	Quantity  float64 `json:"quantity"`
	Source    string  `json:"source"`
	Note      string  `json:"note"`
}
type TodayItem struct {
	Product           Product `json:"product"`
	ScheduledQuantity float64 `json:"scheduledQuantity"`
	TakenQuantity     float64 `json:"takenQuantity"`
	Done              bool    `json:"done"`
	Available         bool    `json:"available"`
	LastIntakeID      string  `json:"lastIntakeId,omitempty"`
}

type dbtx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(value[:4]), hex.EncodeToString(value[4:6]), hex.EncodeToString(value[6:8]), hex.EncodeToString(value[8:10]), hex.EncodeToString(value[10:])), nil
}

func normalizeCreate(input CreateProductInput, now time.Time) (CreateProductInput, core.Schedule, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Brand = strings.TrimSpace(input.Brand)
	input.Unit = strings.TrimSpace(input.Unit)
	if input.Name == "" || len([]rune(input.Name)) > 160 {
		return input, core.Schedule{}, NewError("invalid_product", "产品名称不能为空且不能超过 160 个字符。")
	}
	if input.Unit == "" {
		input.Unit = "粒"
	}
	if len([]rune(input.Unit)) > 40 {
		return input, core.Schedule{}, NewError("invalid_product", "库存单位过长。")
	}
	if input.ProductType == "" {
		input.ProductType = "supplement"
	}
	if input.ProductType != "supplement" && input.ProductType != "otc" && input.ProductType != "prescription" {
		return input, core.Schedule{}, NewError("invalid_product", "产品类型无效。")
	}
	if input.DoseQuantity <= 0 {
		input.DoseQuantity = 1
	}
	if input.DoseTimesPerDay <= 0 {
		input.DoseTimesPerDay = 1
	}
	if input.DoseTimesPerDay > 8 {
		return input, core.Schedule{}, NewError("invalid_schedule", "每日次数不能超过 8 次。")
	}
	if input.IngredientServingQuantity <= 0 {
		input.IngredientServingQuantity = input.DoseQuantity
	}
	if input.RestockThresholdDays < 0 {
		input.RestockThresholdDays = 7
	}
	if input.ExpiryReminderDays < 0 {
		input.ExpiryReminderDays = 30
	}
	if input.OpeningBatch.Quantity <= 0 {
		return input, core.Schedule{}, NewError("invalid_batch", "初始库存必须大于 0。")
	}
	startKey := input.Schedule.StartDate
	if startKey == "" {
		startKey = core.DateKey(now)
	}
	start, err := core.ParseDate(startKey)
	if err != nil {
		return input, core.Schedule{}, NewError("invalid_schedule", "计划开始日期格式无效。")
	}
	input.Schedule.StartDate = startKey
	if len(input.Schedule.Weekdays) == 0 {
		input.Schedule.Weekdays = []int{0, 1, 2, 3, 4, 5, 6}
	}
	dayAnchorKey := input.Schedule.DayCycle.AnchorDate
	if dayAnchorKey == "" {
		dayAnchorKey = startKey
	}
	dayAnchor, err := core.ParseDate(dayAnchorKey)
	if err != nil {
		return input, core.Schedule{}, NewError("invalid_schedule", "日周期锚点格式无效。")
	}
	input.Schedule.DayCycle.AnchorDate = dayAnchorKey
	longStartKey := input.Schedule.LongCycle.StartDate
	if longStartKey == "" {
		longStartKey = startKey
	}
	longStart, err := core.ParseDate(longStartKey)
	if err != nil {
		return input, core.Schedule{}, NewError("invalid_schedule", "长周期开始日期格式无效。")
	}
	input.Schedule.LongCycle.StartDate = longStartKey
	if input.Schedule.DayCycle.CycleDays <= 0 {
		input.Schedule.DayCycle.CycleDays = 28
	}
	if input.Schedule.DayCycle.TakeDays <= 0 {
		input.Schedule.DayCycle.TakeDays = input.Schedule.DayCycle.CycleDays
	}
	if input.Schedule.LongCycle.TakeWeeks <= 0 {
		input.Schedule.LongCycle.TakeWeeks = 6
	}
	if input.Schedule.LongCycle.RestWeeks < 0 {
		return input, core.Schedule{}, NewError("invalid_schedule", "停用周数不能为负数。")
	}
	if len(input.Schedule.ReminderTimes) == 0 {
		input.Schedule.ReminderTimes = []string{"09:00"}
	}
	if len(input.Schedule.ReminderTimes) > 8 {
		return input, core.Schedule{}, NewError("invalid_schedule", "提醒时间不能超过 8 个。")
	}
	for _, value := range input.Schedule.ReminderTimes {
		if !reminderPattern.MatchString(value) {
			return input, core.Schedule{}, NewError("invalid_schedule", "提醒时间必须使用 HH:MM。")
		}
	}
	if input.OpeningBatch.ExpiryDate != "" {
		if _, err = core.ParseDate(input.OpeningBatch.ExpiryDate); err != nil {
			return input, core.Schedule{}, NewError("invalid_batch", "有效期必须使用 YYYY-MM-DD。")
		}
	}
	schedule, err := core.NormalizeSchedule(core.Schedule{StartDate: start, Weekdays: input.Schedule.Weekdays, DayCycle: core.DayCycle{Enabled: input.Schedule.DayCycle.Enabled, CycleDays: input.Schedule.DayCycle.CycleDays, TakeDays: input.Schedule.DayCycle.TakeDays, AnchorDate: dayAnchor}, LongCycle: core.LongCycle{Enabled: input.Schedule.LongCycle.Enabled, TakeWeeks: input.Schedule.LongCycle.TakeWeeks, RestWeeks: input.Schedule.LongCycle.RestWeeks, StartDate: longStart}, ReminderTimes: input.Schedule.ReminderTimes})
	if err != nil {
		return input, core.Schedule{}, NewError("invalid_schedule", err.Error())
	}
	if _, err = core.QuantityFromFloat(input.DoseQuantity); err != nil {
		return input, core.Schedule{}, NewError("invalid_product", "单次用量无效。")
	}
	if _, err = core.QuantityFromFloat(input.OpeningBatch.Quantity); err != nil {
		return input, core.Schedule{}, NewError("invalid_batch", "初始库存无效。")
	}
	return input, schedule, nil
}

func (service *Service) CreateProduct(ctx context.Context, scope Scope, input CreateProductInput) (Product, error) {
	now := service.now()
	normalized, schedule, err := normalizeCreate(input, now)
	if err != nil {
		return Product{}, err
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)
	id, err := service.createProductTx(ctx, tx, scope, normalized, schedule, now)
	if err != nil {
		return Product{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return service.GetProduct(ctx, scope, id)
}

func (service *Service) createProductTx(ctx context.Context, tx pgx.Tx, scope Scope, input CreateProductInput, schedule core.Schedule, now time.Time) (string, error) {
	productID, err := newUUID()
	if err != nil {
		return "", err
	}
	batchID, err := newUUID()
	if err != nil {
		return "", err
	}
	eventID, err := newUUID()
	if err != nil {
		return "", err
	}
	dose, _ := core.QuantityFromFloat(input.DoseQuantity)
	serving, _ := core.QuantityFromFloat(input.IngredientServingQuantity)
	opening, _ := core.QuantityFromFloat(input.OpeningBatch.Quantity)
	if _, err = tx.Exec(ctx, `INSERT INTO products (id,user_id,workspace_id,name,brand,product_type,status,unit,dose_quantity,dose_times_per_day,ingredient_serving_quantity,with_food,restock_threshold_days,expiry_reminder_days,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,'active',$7,$8,$9,$10,$11,$12,$13,$14,$14)`, productID, scope.UserID, scope.WorkspaceID, input.Name, input.Brand, input.ProductType, input.Unit, dose.DatabaseString(), input.DoseTimesPerDay, serving.DatabaseString(), input.WithFood, input.RestockThresholdDays, input.ExpiryReminderDays, now); err != nil {
		return "", err
	}
	weekdays := make([]int16, len(schedule.Weekdays))
	for index, value := range schedule.Weekdays {
		weekdays[index] = int16(value)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO product_schedules (product_id,user_id,workspace_id,version,start_date,weekdays,day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,long_cycle_enabled,long_take_weeks,long_rest_weeks,long_start_date,reminder_times,created_at,updated_at) VALUES ($1,$2,$3,1,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15)`, productID, scope.UserID, scope.WorkspaceID, schedule.StartDate, weekdays, schedule.DayCycle.Enabled, schedule.DayCycle.CycleDays, schedule.DayCycle.TakeDays, schedule.DayCycle.AnchorDate, schedule.LongCycle.Enabled, schedule.LongCycle.TakeWeeks, schedule.LongCycle.RestWeeks, schedule.LongCycle.StartDate, schedule.ReminderTimes, now); err != nil {
		return "", err
	}
	versionID, err := newUUID()
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO day_cycle_versions (id,user_id,workspace_id,product_id,effective_date,enabled,cycle_days,take_days,anchor_date,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, versionID, scope.UserID, scope.WorkspaceID, productID, schedule.DayCycle.AnchorDate, schedule.DayCycle.Enabled, schedule.DayCycle.CycleDays, schedule.DayCycle.TakeDays, schedule.DayCycle.AnchorDate, now); err != nil {
		return "", err
	}
	var expiry any
	if input.OpeningBatch.ExpiryDate != "" {
		parsed, _ := core.ParseDate(input.OpeningBatch.ExpiryDate)
		expiry = parsed
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_batches (id,user_id,workspace_id,product_id,initial_quantity,current_quantity,expiry_date,price_cny,created_at) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8)`, batchID, scope.UserID, scope.WorkspaceID, productID, opening.DatabaseString(), expiry, input.OpeningBatch.PriceCNY, now); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_events (id,user_id,workspace_id,product_id,batch_id,kind,quantity_delta,created_at) VALUES ($1,$2,$3,$4,$5,'opening',$6,$7)`, eventID, scope.UserID, scope.WorkspaceID, productID, batchID, opening.DatabaseString(), now); err != nil {
		return "", err
	}
	for _, ingredient := range input.Ingredients {
		id, createErr := newUUID()
		if createErr != nil {
			return "", createErr
		}
		key := normalizeIngredientKey(ingredient.Key, ingredient.Name)
		name := strings.TrimSpace(ingredient.Name)
		if name == "" {
			name = key
		}
		unit := strings.TrimSpace(ingredient.Unit)
		if unit == "" {
			unit = "mg"
		}
		amount, parseErr := core.QuantityFromFloat(ingredient.Amount)
		if parseErr != nil {
			return "", NewError("invalid_ingredient", "成分剂量无效。")
		}
		if _, err = tx.Exec(ctx, `INSERT INTO product_ingredients (id,user_id,workspace_id,product_id,ingredient_key,name,amount,unit,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, scope.UserID, scope.WorkspaceID, productID, key, name, amount.DatabaseString(), unit, now); err != nil {
			return "", err
		}
	}
	return productID, nil
}

func normalizeIngredientKey(key, name string) string {
	value := strings.ToLower(strings.TrimSpace(key))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(name))
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			return r
		}
		return -1
	}, value)
	if value == "" {
		value = "ingredient"
	}
	return value
}

func (service *Service) GetProduct(ctx context.Context, scope Scope, id string) (Product, error) {
	return loadProduct(ctx, service.pool, scope, id, service.now())
}
func (service *Service) ListProducts(ctx context.Context, scope Scope) ([]Product, error) {
	rows, err := service.pool.Query(ctx, `SELECT id FROM products WHERE user_id=$1 AND workspace_id=$2 AND status<>'archived' ORDER BY created_at DESC LIMIT 100`, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	items := make([]Product, 0, len(ids))
	for _, id := range ids {
		item, loadErr := service.GetProduct(ctx, scope, id)
		if loadErr != nil {
			return nil, loadErr
		}
		items = append(items, item)
	}
	return items, nil
}

func loadProduct(ctx context.Context, db dbtx, scope Scope, id string, now time.Time) (Product, error) {
	var product Product
	var doseText, servingText string
	err := db.QueryRow(ctx, `
		SELECT id,name,brand,product_type,status,unit,dose_quantity::text,dose_times_per_day,
		       ingredient_serving_quantity::text,with_food,restock_threshold_days,expiry_reminder_days,created_at,updated_at
		FROM products WHERE id=$1 AND user_id=$2 AND workspace_id=$3 AND status<>'archived'`, id, scope.UserID, scope.WorkspaceID).
		Scan(&product.ID, &product.Name, &product.Brand, &product.ProductType, &product.Status, &product.Unit, &doseText,
			&product.DoseTimesPerDay, &servingText, &product.WithFood, &product.RestockThresholdDays,
			&product.ExpiryReminderDays, &product.CreatedAt, &product.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	dose, err := core.ParseQuantity(doseText)
	if err != nil {
		return Product{}, err
	}
	serving, err := core.ParseQuantity(servingText)
	if err != nil {
		return Product{}, err
	}
	product.DoseQuantity = dose.Float64()
	product.DailyQuantity = (dose * core.Quantity(product.DoseTimesPerDay)).Float64()
	product.IngredientServingQuantity = serving.Float64()

	var startDate, dayAnchor, longStart time.Time
	var weekdays []int16
	var reminders []string
	var dayEnabled, longEnabled bool
	var dayDays, takeDays, longTake, longRest int
	err = db.QueryRow(ctx, `
		SELECT version,start_date,weekdays,day_cycle_enabled,day_cycle_days,day_take_days,day_anchor_date,
		       long_cycle_enabled,long_take_weeks,long_rest_weeks,long_start_date,reminder_times
		FROM product_schedules WHERE product_id=$1 AND user_id=$2 AND workspace_id=$3`, id, scope.UserID, scope.WorkspaceID).
		Scan(&product.Schedule.Version, &startDate, &weekdays, &dayEnabled, &dayDays, &takeDays, &dayAnchor,
			&longEnabled, &longTake, &longRest, &longStart, &reminders)
	if err != nil {
		return Product{}, err
	}
	weekdayInts := make([]int, len(weekdays))
	for index, value := range weekdays {
		weekdayInts[index] = int(value)
	}
	product.Schedule.StartDate = core.DateKey(startDate)
	product.Schedule.Weekdays = weekdayInts
	product.Schedule.DayCycle = DayCycleInput{Enabled: dayEnabled, CycleDays: dayDays, TakeDays: takeDays, AnchorDate: core.DateKey(dayAnchor)}
	product.Schedule.LongCycle = LongCycleInput{Enabled: longEnabled, TakeWeeks: longTake, RestWeeks: longRest, StartDate: core.DateKey(longStart)}
	product.Schedule.ReminderTimes = reminders

	historyRows, err := db.Query(ctx, `SELECT effective_date,enabled,cycle_days,take_days,anchor_date FROM day_cycle_versions WHERE product_id=$1 AND user_id=$2 AND workspace_id=$3 ORDER BY effective_date`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Product{}, err
	}
	history := []core.DayCycleRule{}
	for historyRows.Next() {
		var rule core.DayCycleRule
		if err = historyRows.Scan(&rule.EffectiveDate, &rule.Enabled, &rule.CycleDays, &rule.TakeDays, &rule.AnchorDate); err != nil {
			historyRows.Close()
			return Product{}, err
		}
		history = append(history, rule)
	}
	err = historyRows.Err()
	historyRows.Close()
	if err != nil {
		return Product{}, err
	}
	schedule, err := core.NormalizeSchedule(core.Schedule{
		StartDate: startDate, Weekdays: weekdayInts,
		DayCycle:  core.DayCycle{Enabled: dayEnabled, CycleDays: dayDays, TakeDays: takeDays, AnchorDate: dayAnchor, History: history},
		LongCycle: core.LongCycle{Enabled: longEnabled, TakeWeeks: longTake, RestWeeks: longRest, StartDate: longStart}, ReminderTimes: reminders,
	})
	if err != nil {
		return Product{}, err
	}

	batchRows, err := db.Query(ctx, `
		SELECT id,initial_quantity::text,current_quantity::text,expiry_date,price_cny::text,created_at
		FROM inventory_batches WHERE product_id=$1 AND user_id=$2 AND workspace_id=$3
		ORDER BY expiry_date ASC NULLS LAST,created_at ASC`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Product{}, err
	}
	product.Batches = []BatchView{}
	var total core.Quantity
	var earliestExpiry *time.Time
	for batchRows.Next() {
		var item BatchView
		var initialText, currentText, priceText string
		var expiry *time.Time
		if err = batchRows.Scan(&item.ID, &initialText, &currentText, &expiry, &priceText, &item.CreatedAt); err != nil {
			batchRows.Close()
			return Product{}, err
		}
		initial, parseErr := core.ParseQuantity(initialText)
		if parseErr != nil {
			batchRows.Close()
			return Product{}, parseErr
		}
		current, parseErr := core.ParseQuantity(currentText)
		if parseErr != nil {
			batchRows.Close()
			return Product{}, parseErr
		}
		price, parseErr := core.ParseQuantity(priceText)
		if parseErr != nil {
			batchRows.Close()
			return Product{}, parseErr
		}
		item.InitialQuantity, item.CurrentQuantity, item.PriceCNY = initial.Float64(), current.Float64(), price.Float64()
		if expiry != nil {
			item.ExpiryDate = core.DateKey(*expiry)
			if current > 0 && earliestExpiry == nil {
				value := expiry.UTC()
				earliestExpiry = &value
			}
		}
		total += current
		product.Batches = append(product.Batches, item)
	}
	err = batchRows.Err()
	batchRows.Close()
	if err != nil {
		return Product{}, err
	}
	product.CurrentQuantity = total.Float64()

	ingredientRows, err := db.Query(ctx, `SELECT id,ingredient_key,name,amount::text,unit FROM product_ingredients WHERE product_id=$1 AND user_id=$2 AND workspace_id=$3 ORDER BY created_at,id`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Product{}, err
	}
	product.Ingredients = []IngredientView{}
	for ingredientRows.Next() {
		var item IngredientView
		var amountText string
		if err = ingredientRows.Scan(&item.ID, &item.Key, &item.Name, &amountText, &item.Unit); err != nil {
			ingredientRows.Close()
			return Product{}, err
		}
		amount, parseErr := core.ParseQuantity(amountText)
		if parseErr != nil {
			ingredientRows.Close()
			return Product{}, parseErr
		}
		item.Amount = amount.Float64()
		product.Ingredients = append(product.Ingredients, item)
	}
	err = ingredientRows.Err()
	ingredientRows.Close()
	if err != nil {
		return Product{}, err
	}
	daily := dose * core.Quantity(product.DoseTimesPerDay)
	product.ExpiryRisk = core.CalculateExpiryRisk(schedule, product.Status, total, daily, earliestExpiry, product.ExpiryReminderDays, now)
	return product, nil
}

func (service *Service) AddBatch(ctx context.Context, scope Scope, productID string, input BatchInput) (Product, error) {
	quantity, err := core.QuantityFromFloat(input.Quantity)
	if err != nil || quantity <= 0 {
		return Product{}, NewError("invalid_batch", "入库数量必须大于 0。")
	}
	var expiry any
	if input.ExpiryDate != "" {
		parsed, parseErr := core.ParseDate(input.ExpiryDate)
		if parseErr != nil {
			return Product{}, NewError("invalid_batch", "有效期必须使用 YYYY-MM-DD。")
		}
		expiry = parsed
	}
	if input.PriceCNY < 0 {
		return Product{}, NewError("invalid_batch", "价格不能为负数。")
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)
	var lockedID string
	if err = tx.QueryRow(ctx, `SELECT id FROM products WHERE id=$1 AND user_id=$2 AND workspace_id=$3 AND status<>'archived' FOR UPDATE`, productID, scope.UserID, scope.WorkspaceID).Scan(&lockedID); errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	} else if err != nil {
		return Product{}, err
	}
	batchID, err := newUUID()
	if err != nil {
		return Product{}, err
	}
	eventID, err := newUUID()
	if err != nil {
		return Product{}, err
	}
	now := service.now()
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_batches (id,user_id,workspace_id,product_id,initial_quantity,current_quantity,expiry_date,price_cny,created_at) VALUES ($1,$2,$3,$4,$5,$5,$6,$7,$8)`, batchID, scope.UserID, scope.WorkspaceID, productID, quantity.DatabaseString(), expiry, input.PriceCNY, now); err != nil {
		return Product{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_events (id,user_id,workspace_id,product_id,batch_id,kind,quantity_delta,created_at) VALUES ($1,$2,$3,$4,$5,'restock',$6,$7)`, eventID, scope.UserID, scope.WorkspaceID, productID, batchID, quantity.DatabaseString(), now); err != nil {
		return Product{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE products SET status=CASE WHEN status='depleted' THEN 'active' ELSE status END,updated_at=$1 WHERE id=$2`, now, productID); err != nil {
		return Product{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return service.GetProduct(ctx, scope, productID)
}

func (service *Service) CreateIntake(ctx context.Context, scope Scope, input CreateIntakeInput, idempotencyKey string) (Intake, Product, error) {
	input.ProductID = strings.TrimSpace(input.ProductID)
	date, err := core.ParseDate(input.Date)
	if err != nil {
		return Intake{}, Product{}, NewError("invalid_intake", "服用日期必须使用 YYYY-MM-DD。")
	}
	quantity, err := core.QuantityFromFloat(input.Quantity)
	if err != nil || quantity <= 0 {
		return Intake{}, Product{}, NewError("invalid_intake", "服用数量必须大于 0。")
	}
	if input.Source == "" {
		input.Source = "scheduled"
	}
	if input.Source != "scheduled" && input.Source != "ad_hoc" && input.Source != "backfill" {
		return Intake{}, Product{}, NewError("invalid_intake", "服用来源无效。")
	}
	if len(input.Note) > 500 || len(idempotencyKey) > 160 {
		return Intake{}, Product{}, NewError("invalid_intake", "请求内容过长。")
	}
	var intakeTime any
	if input.Time != "" {
		parsed, parseErr := time.Parse("15:04", input.Time)
		if parseErr != nil {
			return Intake{}, Product{}, NewError("invalid_intake", "服用时间必须使用 HH:MM。")
		}
		intakeTime = parsed
	}
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Intake{}, Product{}, err
	}
	defer tx.Rollback(ctx)
	if idempotencyKey != "" {
		lockKey := scope.UserID + ":" + scope.WorkspaceID + ":" + idempotencyKey
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
			return Intake{}, Product{}, err
		}
		var existingID string
		lookupErr := tx.QueryRow(ctx, `SELECT id FROM intake_records WHERE user_id=$1 AND workspace_id=$2 AND idempotency_key=$3`, scope.UserID, scope.WorkspaceID, idempotencyKey).Scan(&existingID)
		if lookupErr == nil {
			intake, loadErr := loadIntake(ctx, tx, scope, existingID)
			if loadErr != nil {
				return Intake{}, Product{}, loadErr
			}
			product, loadErr := loadProduct(ctx, tx, scope, intake.ProductID, service.now())
			if loadErr != nil {
				return Intake{}, Product{}, loadErr
			}
			if err = tx.Commit(ctx); err != nil {
				return Intake{}, Product{}, err
			}
			return intake, product, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return Intake{}, Product{}, lookupErr
		}
	}
	intakeID, err := service.createIntakeTx(ctx, tx, scope, input, date, intakeTime, quantity, idempotencyKey, service.now())
	if err != nil {
		return Intake{}, Product{}, err
	}
	intake, err := loadIntake(ctx, tx, scope, intakeID)
	if err != nil {
		return Intake{}, Product{}, err
	}
	product, err := loadProduct(ctx, tx, scope, input.ProductID, service.now())
	if err != nil {
		return Intake{}, Product{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Intake{}, Product{}, err
	}
	return intake, product, nil
}

func (service *Service) createIntakeTx(ctx context.Context, tx pgx.Tx, scope Scope, input CreateIntakeInput, date time.Time, intakeTime any, quantity core.Quantity, idempotencyKey string, now time.Time) (string, error) {
	var productID string
	if err := tx.QueryRow(ctx, `SELECT id FROM products WHERE id=$1 AND user_id=$2 AND workspace_id=$3 AND status<>'archived' FOR UPDATE`, input.ProductID, scope.UserID, scope.WorkspaceID).Scan(&productID); errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	} else if err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx, `SELECT id,initial_quantity::text,current_quantity::text,expiry_date,price_cny::text,created_at FROM inventory_batches WHERE product_id=$1 AND user_id=$2 AND workspace_id=$3 AND current_quantity>0 ORDER BY expiry_date ASC NULLS LAST,created_at ASC FOR UPDATE`, productID, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return "", err
	}
	batches := []core.Batch{}
	for rows.Next() {
		var batch core.Batch
		var initialText, currentText, priceText string
		if err = rows.Scan(&batch.ID, &initialText, &currentText, &batch.Expiry, &priceText, &batch.CreatedAt); err != nil {
			rows.Close()
			return "", err
		}
		batch.Initial, err = core.ParseQuantity(initialText)
		if err != nil {
			rows.Close()
			return "", err
		}
		batch.Current, err = core.ParseQuantity(currentText)
		if err != nil {
			rows.Close()
			return "", err
		}
		priceCNY, parseErr := strconv.ParseFloat(priceText, 64)
		if parseErr != nil {
			rows.Close()
			return "", parseErr
		}
		batch.PriceCents = int64(math.Round(priceCNY * 100))
		batches = append(batches, batch)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	allocations, err := core.AllocateFEFO(batches, quantity)
	if err != nil {
		return "", ErrInsufficientInventory
	}
	intakeID, err := newUUID()
	if err != nil {
		return "", err
	}
	var key any
	if idempotencyKey != "" {
		key = idempotencyKey
	}
	if _, err = tx.Exec(ctx, `INSERT INTO intake_records (id,user_id,workspace_id,product_id,intake_date,intake_time,quantity,source,status,idempotency_key,note,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'active',$9,$10,$11)`, intakeID, scope.UserID, scope.WorkspaceID, productID, date, intakeTime, quantity.DatabaseString(), input.Source, key, input.Note, now); err != nil {
		return "", err
	}
	for _, allocation := range allocations {
		result, updateErr := tx.Exec(ctx, `UPDATE inventory_batches SET current_quantity=current_quantity-$1 WHERE id=$2 AND user_id=$3 AND workspace_id=$4 AND current_quantity>=$1`, allocation.Quantity.DatabaseString(), allocation.BatchID, scope.UserID, scope.WorkspaceID)
		if updateErr != nil {
			return "", updateErr
		}
		if result.RowsAffected() != 1 {
			return "", ErrInsufficientInventory
		}
		if _, err = tx.Exec(ctx, `INSERT INTO intake_allocations (intake_id,batch_id,quantity,unit_cost_cny) VALUES ($1,$2,$3,$4)`, intakeID, allocation.BatchID, allocation.Quantity.DatabaseString(), allocation.UnitCostCNY); err != nil {
			return "", err
		}
		eventID, createErr := newUUID()
		if createErr != nil {
			return "", createErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO inventory_events (id,user_id,workspace_id,product_id,batch_id,intake_id,kind,quantity_delta,created_at) VALUES ($1,$2,$3,$4,$5,$6,'intake',$7,$8)`, eventID, scope.UserID, scope.WorkspaceID, productID, allocation.BatchID, intakeID, (-allocation.Quantity).DatabaseString(), now); err != nil {
			return "", err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE products SET status=CASE WHEN (SELECT COALESCE(sum(current_quantity),0) FROM inventory_batches WHERE product_id=$2 AND user_id=$3 AND workspace_id=$4)=0 THEN 'depleted' ELSE CASE WHEN status='depleted' THEN 'active' ELSE status END END,updated_at=$1 WHERE id=$2`, now, productID, scope.UserID, scope.WorkspaceID); err != nil {
		return "", err
	}
	return intakeID, nil
}

func loadIntake(ctx context.Context, db dbtx, scope Scope, id string) (Intake, error) {
	var intake Intake
	var date time.Time
	var quantityText string
	var timeText string
	err := db.QueryRow(ctx, `SELECT id,product_id,intake_date,COALESCE(to_char(intake_time,'HH24:MI'),''),quantity::text,source,status,created_at,revoked_at FROM intake_records WHERE id=$1 AND user_id=$2 AND workspace_id=$3`, id, scope.UserID, scope.WorkspaceID).Scan(&intake.ID, &intake.ProductID, &date, &timeText, &quantityText, &intake.Source, &intake.Status, &intake.CreatedAt, &intake.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Intake{}, ErrNotFound
	}
	if err != nil {
		return Intake{}, err
	}
	quantity, err := core.ParseQuantity(quantityText)
	if err != nil {
		return Intake{}, err
	}
	intake.Date, intake.Time, intake.Quantity = core.DateKey(date), timeText, quantity.Float64()
	rows, err := db.Query(ctx, `SELECT ia.batch_id,ia.quantity::text,ia.unit_cost_cny::text FROM intake_allocations ia JOIN inventory_batches b ON b.id=ia.batch_id WHERE ia.intake_id=$1 AND b.user_id=$2 AND b.workspace_id=$3 ORDER BY b.created_at`, id, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return Intake{}, err
	}
	defer rows.Close()
	intake.Allocations = []AllocationView{}
	for rows.Next() {
		var item AllocationView
		var quantityValue, costValue string
		if err = rows.Scan(&item.BatchID, &quantityValue, &costValue); err != nil {
			return Intake{}, err
		}
		quantity, parseErr := core.ParseQuantity(quantityValue)
		if parseErr != nil {
			return Intake{}, parseErr
		}
		cost, parseErr := core.ParseQuantity(costValue)
		if parseErr != nil {
			return Intake{}, parseErr
		}
		item.Quantity, item.UnitCostCNY = quantity.Float64(), cost.Float64()
		intake.Allocations = append(intake.Allocations, item)
	}
	return intake, rows.Err()
}

func (service *Service) UndoIntake(ctx context.Context, scope Scope, id string) (Intake, Product, error) {
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Intake{}, Product{}, err
	}
	defer tx.Rollback(ctx)
	var productID, status string
	if err = tx.QueryRow(ctx, `SELECT product_id,status FROM intake_records WHERE id=$1 AND user_id=$2 AND workspace_id=$3 FOR UPDATE`, id, scope.UserID, scope.WorkspaceID).Scan(&productID, &status); errors.Is(err, pgx.ErrNoRows) {
		return Intake{}, Product{}, ErrNotFound
	} else if err != nil {
		return Intake{}, Product{}, err
	}
	if status == "active" {
		rows, queryErr := tx.Query(ctx, `SELECT ia.batch_id,ia.quantity::text FROM intake_allocations ia JOIN inventory_batches b ON b.id=ia.batch_id WHERE ia.intake_id=$1 AND b.user_id=$2 AND b.workspace_id=$3 ORDER BY b.id FOR UPDATE OF b`, id, scope.UserID, scope.WorkspaceID)
		if queryErr != nil {
			return Intake{}, Product{}, queryErr
		}
		type restoration struct {
			batchID  string
			quantity core.Quantity
		}
		restores := []restoration{}
		for rows.Next() {
			var item restoration
			var quantityText string
			if queryErr = rows.Scan(&item.batchID, &quantityText); queryErr != nil {
				rows.Close()
				return Intake{}, Product{}, queryErr
			}
			item.quantity, queryErr = core.ParseQuantity(quantityText)
			if queryErr != nil {
				rows.Close()
				return Intake{}, Product{}, queryErr
			}
			restores = append(restores, item)
		}
		queryErr = rows.Err()
		rows.Close()
		if queryErr != nil {
			return Intake{}, Product{}, queryErr
		}
		now := service.now()
		for _, item := range restores {
			result, updateErr := tx.Exec(ctx, `UPDATE inventory_batches SET current_quantity=current_quantity+$1 WHERE id=$2 AND user_id=$3 AND workspace_id=$4 AND current_quantity+$1<=initial_quantity`, item.quantity.DatabaseString(), item.batchID, scope.UserID, scope.WorkspaceID)
			if updateErr != nil {
				return Intake{}, Product{}, updateErr
			}
			if result.RowsAffected() != 1 {
				return Intake{}, Product{}, fmt.Errorf("restore would exceed initial quantity")
			}
			eventID, createErr := newUUID()
			if createErr != nil {
				return Intake{}, Product{}, createErr
			}
			if _, updateErr = tx.Exec(ctx, `INSERT INTO inventory_events (id,user_id,workspace_id,product_id,batch_id,intake_id,kind,quantity_delta,created_at) VALUES ($1,$2,$3,$4,$5,$6,'undo',$7,$8)`, eventID, scope.UserID, scope.WorkspaceID, productID, item.batchID, id, item.quantity.DatabaseString(), now); updateErr != nil {
				return Intake{}, Product{}, updateErr
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE intake_records SET status='revoked',revoked_at=$1 WHERE id=$2`, now, id); err != nil {
			return Intake{}, Product{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE products SET status=CASE WHEN status='depleted' THEN 'active' ELSE status END,updated_at=$1 WHERE id=$2`, now, productID); err != nil {
			return Intake{}, Product{}, err
		}
	}
	intake, err := loadIntake(ctx, tx, scope, id)
	if err != nil {
		return Intake{}, Product{}, err
	}
	product, err := loadProduct(ctx, tx, scope, productID, service.now())
	if err != nil {
		return Intake{}, Product{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Intake{}, Product{}, err
	}
	return intake, product, nil
}

func (service *Service) Today(ctx context.Context, scope Scope, dateKey string) ([]TodayItem, error) {
	date, err := core.ParseDate(dateKey)
	if err != nil {
		return nil, NewError("invalid_date", "日期必须使用 YYYY-MM-DD。")
	}
	products, err := service.ListProducts(ctx, scope)
	if err != nil {
		return nil, err
	}
	type takenState struct {
		quantity core.Quantity
		lastID   string
	}
	states := map[string]takenState{}
	rows, err := service.pool.Query(ctx, `SELECT product_id,COALESCE(sum(quantity),0)::text,(array_agg(id ORDER BY created_at DESC))[1] FROM intake_records WHERE user_id=$1 AND workspace_id=$2 AND intake_date=$3 AND status='active' GROUP BY product_id`, scope.UserID, scope.WorkspaceID, date)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var productID, quantityText, lastID string
		if err = rows.Scan(&productID, &quantityText, &lastID); err != nil {
			rows.Close()
			return nil, err
		}
		quantity, parseErr := core.ParseQuantity(quantityText)
		if parseErr != nil {
			rows.Close()
			return nil, parseErr
		}
		states[productID] = takenState{quantity: quantity, lastID: lastID}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	items := []TodayItem{}
	for _, product := range products {
		if product.Status == "paused" {
			continue
		}
		schedule, parseErr := scheduleFromView(product.Schedule)
		if parseErr != nil {
			return nil, parseErr
		}
		match := core.MatchSchedule(schedule, "active", core.Quantity(1), date)
		if !match.Active {
			continue
		}
		dose, _ := core.QuantityFromFloat(product.DoseQuantity)
		scheduled := dose * core.Quantity(product.DoseTimesPerDay)
		state := states[product.ID]
		current, _ := core.QuantityFromFloat(product.CurrentQuantity)
		remaining := scheduled - state.quantity
		if remaining < 0 {
			remaining = 0
		}
		items = append(items, TodayItem{Product: product, ScheduledQuantity: scheduled.Float64(), TakenQuantity: state.quantity.Float64(), Done: state.quantity >= scheduled, Available: current >= remaining, LastIntakeID: state.lastID})
	}
	return items, nil
}

func scheduleFromView(view ScheduleView) (core.Schedule, error) {
	start, err := core.ParseDate(view.StartDate)
	if err != nil {
		return core.Schedule{}, err
	}
	dayAnchor, err := core.ParseDate(view.DayCycle.AnchorDate)
	if err != nil {
		return core.Schedule{}, err
	}
	longStart, err := core.ParseDate(view.LongCycle.StartDate)
	if err != nil {
		return core.Schedule{}, err
	}
	return core.NormalizeSchedule(core.Schedule{StartDate: start, Weekdays: view.Weekdays, DayCycle: core.DayCycle{Enabled: view.DayCycle.Enabled, CycleDays: view.DayCycle.CycleDays, TakeDays: view.DayCycle.TakeDays, AnchorDate: dayAnchor}, LongCycle: core.LongCycle{Enabled: view.LongCycle.Enabled, TakeWeeks: view.LongCycle.TakeWeeks, RestWeeks: view.LongCycle.RestWeeks, StartDate: longStart}, ReminderTimes: view.ReminderTimes})
}

func (service *Service) EnsureDemo(ctx context.Context, scope Scope) error {
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "demo-seed:"+scope.WorkspaceID); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM products WHERE user_id=$1 AND workspace_id=$2`, scope.UserID, scope.WorkspaceID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if err = service.SeedDemo(ctx, tx, scope.UserID, scope.WorkspaceID, service.now()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (service *Service) SeedDemo(ctx context.Context, tx pgx.Tx, userID, workspaceID string, now time.Time) error {
	scope := Scope{UserID: userID, WorkspaceID: workspaceID}
	start := core.DateKey(now)
	weekday := int(now.Weekday())
	withFood := true
	inputs := []CreateProductInput{
		{Name: "维生素 D3", Brand: "演示品牌", Unit: "粒", DoseQuantity: 1, DoseTimesPerDay: 1, IngredientServingQuantity: 1, WithFood: &withFood, RestockThresholdDays: 7, ExpiryReminderDays: 30, Schedule: ScheduleInput{StartDate: start, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, ReminderTimes: []string{"08:30"}}, OpeningBatch: BatchInput{Quantity: 28, ExpiryDate: core.DateKey(now.AddDate(0, 5, 0)), PriceCNY: 79}, Ingredients: []IngredientInput{{Key: "vitamin-d3", Name: "维生素 D3", Amount: 25, Unit: "μg"}}},
		{Name: "镁", Brand: "演示品牌", Unit: "粒", DoseQuantity: 2, DoseTimesPerDay: 1, IngredientServingQuantity: 2, RestockThresholdDays: 7, ExpiryReminderDays: 30, Schedule: ScheduleInput{StartDate: start, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, ReminderTimes: []string{"22:30"}}, OpeningBatch: BatchInput{Quantity: 42, ExpiryDate: core.DateKey(now.AddDate(0, 7, 0)), PriceCNY: 99}, Ingredients: []IngredientInput{{Key: "magnesium", Name: "镁", Amount: 200, Unit: "mg"}}},
		{Name: "鱼油", Brand: "演示品牌", Unit: "粒", DoseQuantity: 2, DoseTimesPerDay: 1, IngredientServingQuantity: 2, RestockThresholdDays: 7, ExpiryReminderDays: 30, Schedule: ScheduleInput{StartDate: start, Weekdays: []int{weekday}, ReminderTimes: []string{"12:30"}}, OpeningBatch: BatchInput{Quantity: 12, ExpiryDate: core.DateKey(now.AddDate(0, 1, 0)), PriceCNY: 129}, Ingredients: []IngredientInput{{Key: "epa-dha", Name: "EPA + DHA", Amount: 600, Unit: "mg"}}},
	}
	for _, input := range inputs {
		normalized, schedule, err := normalizeCreate(input, now)
		if err != nil {
			return err
		}
		if _, err = service.createProductTx(ctx, tx, scope, normalized, schedule, now); err != nil {
			return err
		}
	}
	return nil
}
