package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
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

// If your DB enums are strings (recommended), keep these as string-based types.
// Expand the consts to match your enum values.
type AggKind string

const (
	AggSum     AggKind = "sum"
	AggBoolean AggKind = "boolean"
	AggCount   AggKind = "count"
)

func ToAggKind(s string) (AggKind, error) {
	switch AggKind(s) {
	case AggSum, AggBoolean, AggCount:
		return AggKind(s), nil
	default:
		return "", fmt.Errorf("unrecognized aggregate kind %s", s)
	}
}

type PeriodType string

const (
	PeriodDaily   PeriodType = "daily"
	PeriodWeekly  PeriodType = "weekly"
	PeriodMonthly PeriodType = "monthly"
	PeriodRolling PeriodType = "rolling"
)

func ToPeriodType(s string) (PeriodType, error) {
	switch PeriodType(s) {
	case PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodRolling:
		return PeriodType(s), nil
	default:
		return "", fmt.Errorf("unrecognized period type %s", s)
	}
}

const (
	HumanDateFormat  = "Jan 1, 2006 at 3:04pm"
	ToFrontEndFormat = "2006-01-02T15:04"
)

// ---------- app_user ----------
type AppUser struct {
	ID           int64     `db:"id"            json:"id"`
	Email        string    `db:"email"         json:"email"`         // CITEXT -> string
	Username     string    `db:"username"      json:"username"`      // VARCHAR(50) UNIQUE NOT NULL
	PasswordHash string    `db:"password_hash" json:"password_hash"` // VARCHAR(255) NOT NULL
	TZ           string    `db:"tz"            json:"tz"`            // NOT NULL, default 'America/Toronto'
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

// ---------- user_sessions ----------
type UserSession struct {
	ID           string    `db:"id"            json:"id"`
	UserID       int64     `db:"user_id"       json:"user_id"`
	SessionToken string    `db:"session_token" json:"session_token"`
	ExpiresAt    time.Time `db:"expires_at"    json:"expires_at"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// ---------- habit ----------
type Habit struct {
	ID               int64           `db:"id"                   json:"id"`
	UserID           int64           `db:"user_id"              json:"user_id"`
	Name             string          `db:"name"                 json:"name"`
	UnitLabel        sql.NullString  `db:"unit_label"           json:"unit_label,omitempty"`       // nullable
	Agg              AggKind         `db:"agg"                  json:"agg"`                        // NOT NULL, default 'sum'
	TargetPerPeriod  decimal.Decimal `db:"target_per_period"    json:"target_per_period"`          // NUMERIC(12,2)
	PerLogDefaultQty decimal.Decimal `db:"per_log_default_qty"  json:"per_log_default_qty"`        // NUMERIC(12,2)
	Period           PeriodType      `db:"period"               json:"period"`                     // NOT NULL, default 'daily'
	WeekStartDOW     int32           `db:"week_start_dow"       json:"week_start_dow"`             // 0..6
	MonthAnchorDay   int32           `db:"month_anchor_day"     json:"month_anchor_day"`           // 1..28
	RollingLenDays   sql.NullInt32   `db:"rolling_len_days"     json:"rolling_len_days,omitempty"` // nullable, >= 1
	AnchorDate       time.Time       `db:"anchor_date"          json:"anchor_date"`                // DATE (use time.Date w/ midnight)
	TZOverride       sql.NullString  `db:"tz"                   json:"tz_override,omitempty"`      // nullable override
	IsActive         bool            `db:"is_active"            json:"is_active"`
	CreatedAt        time.Time       `db:"created_at"           json:"created_at"`
}

// ---------- habit_log ----------
type HabitLog struct {
	ID         int64           `db:"id"          json:"id"`
	HabitID    int64           `db:"habit_id"    json:"habit_id"`
	OccurredAt time.Time       `db:"occurred_at" json:"occurred_at"` // store UTC; UI collects in user TZ
	Quantity   decimal.Decimal `db:"quantity"    json:"quantity"`    // NUMERIC(12,2), >= 0
	Note       sql.NullString  `db:"note"        json:"note,omitempty"`
	CreatedAt  time.Time       `db:"created_at"  json:"created_at"`
}
