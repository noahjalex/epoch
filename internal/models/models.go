package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// JSONB is a custom type for handling PostgreSQL JSONB fields
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// User represents a user in the system
type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Tz        string    `json:"tz"`
	CreatedAt time.Time `json:"created_at"`
}

// Habit represents a habit identity
type Habit struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Slug       string    `json:"slug"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
}

// HabitVersion represents a versioned habit definition
type HabitVersion struct {
	ID              int64      `json:"id"`
	HabitID         int64      `json:"habit_id"`
	Version         int        `json:"version"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	Category        *string    `json:"category"`
	Polarity        string     `json:"polarity"`
	ScheduleRrule   *string    `json:"schedule_rrule"`
	DailyExpectJSON JSONB      `json:"daily_expect_json"`
	ActiveFrom      time.Time  `json:"active_from"`
	ActiveTo        *time.Time `json:"active_to"`
}

// Unit represents a measurement unit
type Unit struct {
	ID           int64   `json:"id"`
	Code         string  `json:"code"`
	QuantityKind string  `json:"quantity_kind"`
	ToBaseFactor float64 `json:"to_base_factor"`
	BaseUnitCode string  `json:"base_unit_code"`
}

// HabitMetric represents a metric identity for a habit
type HabitMetric struct {
	ID        int64     `json:"id"`
	HabitID   int64     `json:"habit_id"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// HabitMetricVersion represents a versioned metric definition
type HabitMetricVersion struct {
	ID             int64      `json:"id"`
	MetricID       int64      `json:"metric_id"`
	Version        int        `json:"version"`
	Name           string     `json:"name"`
	MetricKind     string     `json:"metric_kind"`
	UnitID         *int64     `json:"unit_id"`
	Polarity       *string    `json:"polarity"`
	AggKindDefault string     `json:"agg_kind_default"`
	MinValue       *float64   `json:"min_value"`
	MaxValue       *float64   `json:"max_value"`
	IsRequired     bool       `json:"is_required"`
	ActiveFrom     time.Time  `json:"active_from"`
	ActiveTo       *time.Time `json:"active_to"`
	Metadata       JSONB      `json:"metadata"`
}

// HabitLog represents a logged habit event
type HabitLog struct {
	ID              int64     `json:"id"`
	HabitID         int64     `json:"habit_id"`
	HabitVersionID  int64     `json:"habit_version_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	LocalDay        time.Time `json:"local_day"`
	Tz              string    `json:"tz"`
	Note            *string   `json:"note"`
	SupersedesLogID *int64    `json:"supersedes_log_id"`
	Source          *string   `json:"source"`
	IdempotencyKey  *string   `json:"idempotency_key"`
	CreatedAt       time.Time `json:"created_at"`
}

// HabitLogValue represents metric values for a log entry
type HabitLogValue struct {
	ID              int64    `json:"id"`
	LogID           int64    `json:"log_id"`
	MetricID        int64    `json:"metric_id"`
	MetricVersionID int64    `json:"metric_version_id"`
	ValueBool       *bool    `json:"value_bool"`
	ValueNum        *float64 `json:"value_num"`
	Metadata        JSONB    `json:"metadata"`
}

// HabitTarget represents goals/targets for habits
type HabitTarget struct {
	ID          int64      `json:"id"`
	HabitID     int64      `json:"habit_id"`
	MetricID    *int64     `json:"metric_id"`
	Period      string     `json:"period"`
	AggKind     string     `json:"agg_kind"`
	TargetValue *float64   `json:"target_value"`
	TargetBool  *bool      `json:"target_bool"`
	ActiveFrom  time.Time  `json:"active_from"`
	ActiveTo    *time.Time `json:"active_to"`
}

// HabitRollupDaily represents daily aggregated data
type HabitRollupDaily struct {
	HabitID    int64     `json:"habit_id"`
	LocalDay   time.Time `json:"local_day"`
	MetricID   *int64    `json:"metric_id"`
	ValueNum   *float64  `json:"value_num"`
	ValueBool  *bool     `json:"value_bool"`
	ComputedAt time.Time `json:"computed_at"`
}

// HabitWithDetails combines habit with its current version and metrics
type HabitWithDetails struct {
	Habit   Habit               `json:"habit"`
	Version HabitVersion        `json:"version"`
	Metrics []MetricWithVersion `json:"metrics"`
	Targets []HabitTarget       `json:"targets"`
}

// MetricWithVersion combines metric with its current version
type MetricWithVersion struct {
	Metric  HabitMetric        `json:"metric"`
	Version HabitMetricVersion `json:"version"`
	Unit    *Unit              `json:"unit"`
}

// LogRequest represents the request payload for logging habit data
type LogRequest struct {
	OccurredAt time.Time  `json:"occurred_at"`
	Tz         string     `json:"tz"`
	Values     []LogValue `json:"values"`
	Note       string     `json:"note"`
}

// LogValue represents a single metric value in a log request
type LogValue struct {
	MetricSlug string   `json:"metric_slug"`
	ValueNum   *float64 `json:"value_num"`
	ValueBool  *bool    `json:"value_bool"`
}

// DashboardData represents aggregated data for the dashboard
type DashboardData struct {
	Habits     []HabitWithDetails `json:"habits"`
	RecentLogs []LogWithDetails   `json:"recent_logs"`
	Streaks    []StreakData       `json:"streaks"`
}

// LogWithDetails represents a log with its associated habit and metric details
type LogWithDetails struct {
	Log    HabitLog         `json:"log"`
	Habit  HabitWithDetails `json:"habit"`
	Values []LogValueDetail `json:"values"`
}

// LogValueDetail represents a log value with metric details
type LogValueDetail struct {
	Value  HabitLogValue     `json:"value"`
	Metric MetricWithVersion `json:"metric"`
}

// StreakData represents streak information for a habit
type StreakData struct {
	HabitID       int64  `json:"habit_id"`
	HabitName     string `json:"habit_name"`
	CurrentStreak int    `json:"current_streak"`
	BestStreak    int    `json:"best_streak"`
}
