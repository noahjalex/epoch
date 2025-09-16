package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Repo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

// -------------------- USERS --------------------

func (r *Repo) CreateUser(ctx context.Context, username, email, passwordHash, tz string) (*AppUser, error) {
	var u AppUser
	err := r.db.GetContext(ctx, &u, `
		INSERT INTO app_user (username, email, password_hash, tz)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4,''), 'America/Toronto'))
		RETURNING id, username, email, password_hash, tz, created_at
	`, username, email, passwordHash, tz)
	return &u, err
}

func (r *Repo) GetUserByUsername(ctx context.Context, username string) (*AppUser, error) {
	var u AppUser
	err := r.db.GetContext(ctx, &u, `
		SELECT id, username, email, password_hash, tz, created_at
		FROM app_user
		WHERE username = $1
	`, username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (*AppUser, error) {
	var u AppUser
	err := r.db.GetContext(ctx, &u, `
		SELECT id, username, email, password_hash, tz, created_at
		FROM app_user
		WHERE email = $1
	`, email)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) GetUser(ctx context.Context, userID int64) (*AppUser, error) {
	var u AppUser
	err := r.db.GetContext(ctx, &u, `
		SELECT id, username, email, password_hash, tz, created_at
		FROM app_user
		WHERE id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// -------------------- SESSIONS --------------------

func (r *Repo) CreateSession(ctx context.Context, userID int64, sessionToken string, expiresAt time.Time) (*UserSession, error) {
	var s UserSession
	err := r.db.GetContext(ctx, &s, `
		INSERT INTO user_sessions (user_id, session_token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, session_token, expires_at, created_at, updated_at
	`, userID, sessionToken, expiresAt)
	return &s, err
}

func (r *Repo) GetSessionByToken(ctx context.Context, sessionToken string) (*UserSession, error) {
	var s UserSession
	err := r.db.GetContext(ctx, &s, `
		SELECT id, user_id, session_token, expires_at, created_at, updated_at
		FROM user_sessions
		WHERE session_token = $1
	`, sessionToken)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) DeleteSession(ctx context.Context, sessionToken string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM user_sessions WHERE session_token = $1
	`, sessionToken)
	return err
}

func (r *Repo) DeleteExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM user_sessions WHERE expires_at < NOW()
	`)
	return err
}

func (r *Repo) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM user_sessions WHERE user_id = $1
	`, userID)
	return err
}

// -------------------- HABITS --------------------

func (r *Repo) CreateHabit(ctx context.Context, h *Habit) (*Habit, error) {
	// Let DB defaults apply when zero-values are passed (e.g., agg, period)
	query := `
		INSERT INTO habit (
			user_id, name, unit_label, agg, target_per_period, per_log_default_qty,
			period, week_start_dow, month_anchor_day, rolling_len_days, anchor_date, tz, is_active
		) VALUES (
			:user_id, :name, :unit_label, :agg, :target_per_period, :per_log_default_qty,
			:period, :week_start_dow, :month_anchor_day, :rolling_len_days, :anchor_date, :tz, :is_active
		)
		RETURNING id, user_id, name, unit_label, agg, target_per_period, per_log_default_qty,
		          period, week_start_dow, month_anchor_day, rolling_len_days, anchor_date, tz, is_active, created_at
	`
	// sqlx.NamedExec/Query requires named params; we can pass the struct directly.
	rows, err := r.db.NamedQueryContext(ctx, query, h)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var out Habit
		if err := rows.StructScan(&out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return nil, errors.New("no row returned")
}

func (r *Repo) GetHabit(ctx context.Context, habitID int64) (*Habit, error) {
	var h Habit
	err := r.db.GetContext(ctx, &h, `
		SELECT id, user_id, name, unit_label, agg, target_per_period, per_log_default_qty,
		       period, week_start_dow, month_anchor_day, rolling_len_days, anchor_date, tz, is_active, created_at
		FROM habit
		WHERE id = $1
	`, habitID)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *Repo) ListHabitsByUser(ctx context.Context, userID int64, activeOnly bool) ([]Habit, error) {
	q := `
		SELECT id, user_id, name, unit_label, agg, target_per_period, per_log_default_qty,
		       period, week_start_dow, month_anchor_day, rolling_len_days, anchor_date, tz, is_active, created_at
		FROM habit
		WHERE user_id = $1
	`
	if activeOnly {
		q += " AND is_active = TRUE"
	}
	q += " ORDER BY created_at DESC"

	var hs []Habit
	if err := r.db.SelectContext(ctx, &hs, q, userID); err != nil {
		return nil, err
	}
	return hs, nil
}

func (r *Repo) DeactivateHabit(ctx context.Context, habitID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE habit SET is_active = FALSE WHERE id = $1
	`, habitID)
	return err
}

func (r *Repo) UpdateHabit(ctx context.Context, h *Habit) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE habit
		SET
			name = $1,
			unit_label = $2,
			agg = $3,
			target_per_period = $4,
			per_log_default_qty = $5,
			period = $6,
			week_start_dow = $7,
			month_anchor_day = $8,
			rolling_len_days = $9,
			anchor_date = $10,
			tz = $11,
			is_active = $12
		WHERE id = $13
			AND user_id = $14
	`, h.Name,
		h.UnitLabel,
		h.Agg,
		h.TargetPerPeriod,
		h.PerLogDefaultQty,
		h.Period,
		h.WeekStartDOW,
		h.MonthAnchorDay,
		h.RollingLenDays,
		h.AnchorDate,
		h.TZOverride,
		h.IsActive,
		h.ID,
		h.UserID,
	)
	return err

}

// -------------------- LOGS --------------------

func (r *Repo) InsertLog(ctx context.Context, l *HabitLog) (*HabitLog, error) {

	query := `
		INSERT INTO habit_log (habit_id, occurred_at, quantity, note)
		VALUES (:habit_id, :occurred_at, :quantity, :note)
		RETURNING id, habit_id, occurred_at, quantity, note, created_at
	`
	rows, err := r.db.NamedQueryContext(ctx, query, l)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var out HabitLog
		if err := rows.StructScan(&out); err != nil {
			return nil, err
		}
		return &out, nil
	}
	return nil, errors.New("no row returned")
}

func (r *Repo) ListLogsWithin(ctx context.Context, habitID int64, start, end time.Time) ([]HabitLog, error) {
	var ls []HabitLog
	err := r.db.SelectContext(ctx, &ls, `
		SELECT id, habit_id, occurred_at, quantity, note, created_at
		FROM habit_log
		WHERE habit_id = $1
		  AND occurred_at >= $2
		  AND occurred_at <  $3
		ORDER BY occurred_at ASC, id ASC
	`, habitID, start, end)
	return ls, err
}

func (r *Repo) ListLogs(ctx context.Context, habitID int64) ([]HabitLog, error) {
	var ls []HabitLog
	err := r.db.SelectContext(ctx, &ls, `
		SELECT id, habit_id, occurred_at, quantity, note, created_at
		FROM habit_log
		WHERE habit_id = $1
		ORDER BY occurred_at ASC, id ASC
	`, habitID)
	return ls, err
}

func (r *Repo) DeleteLog(ctx context.Context, logID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM habit_log WHERE id = $1`, logID)
	return err
}

func (r *Repo) UpdateLog(ctx context.Context, l *HabitLog) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE habit_log
		SET habit_id = $1, occurred_at = $2, quantity = $3, note = $4
		WHERE id = $5
	`, l.HabitID, l.OccurredAt, l.Quantity, l.Note, l.ID)
	return err
}

func (r *Repo) DeleteHabit(ctx context.Context, habitID int64) error {
	// Delete logs first due to foreign key constraint
	_, err := r.db.ExecContext(ctx, `DELETE FROM habit_log WHERE habit_id = $1`, habitID)
	if err != nil {
		return err
	}

	// Delete the habit
	_, err = r.db.ExecContext(ctx, `DELETE FROM habit WHERE id = $1`, habitID)
	return err
}

// -------------------- ROLLUP / BUCKETS (for charts) --------------------

type BucketRow struct {
	BucketStart   time.Time       `db:"bucket_start"    json:"bucket_start"`
	BucketEnd     time.Time       `db:"bucket_end"      json:"bucket_end"`
	Value         decimal.Decimal `db:"value"           json:"value"`
	Target        decimal.Decimal `db:"target_per_period" json:"target"`
	ProgressRatio sql.NullFloat64 `db:"progress_ratio"  json:"progress_ratio,omitempty"`
}

// RollupBuckets emits continuous buckets in [start,end] for the given habit,
// computing aggregated value, target, and progress ratio. Aligns to habit/user tz,
// handles daily/weekly/monthly/rolling and fills gaps (0 values).
func (r *Repo) RollupBuckets(ctx context.Context, habitID int64, start, end time.Time) ([]BucketRow, error) {
	// NOTE: This SQL mirrors the earlier design. If you extend agg_kind beyond sum/count/boolean,
	// add additional WHEN branches in values_in_bucket CASE below.
	sql := `
WITH params AS (
  SELECT
    h.id,
    h.agg,
    h.target_per_period,
    h.period,
    h.week_start_dow,
    h.rolling_len_days,
    h.anchor_date,
    COALESCE(h.tz, u.tz) AS tz
  FROM habit h
  JOIN app_user u ON u.id = h.user_id
  WHERE h.id = $1
),
buckets AS (
  SELECT
    CASE p.period
      WHEN 'daily'   THEN generate_series(date_trunc('day', $2 AT TIME ZONE p.tz),
                                          date_trunc('day', $3 AT TIME ZONE p.tz),
                                          INTERVAL '1 day')
      WHEN 'weekly'  THEN generate_series(
                          date_trunc('week', ($2 AT TIME ZONE p.tz) - ((p.week_start_dow - 1) * INTERVAL '1 day'))
                          + ((p.week_start_dow - 1) * INTERVAL '1 day'),
                          date_trunc('week', ($3 AT TIME ZONE p.tz) - ((p.week_start_dow - 1) * INTERVAL '1 day'))
                          + ((p.week_start_dow - 1) * INTERVAL '1 day'),
                          INTERVAL '7 days')
      WHEN 'monthly' THEN generate_series(date_trunc('month', $2 AT TIME ZONE p.tz),
                                          date_trunc('month', $3 AT TIME ZONE p.tz),
                                          INTERVAL '1 month')
      WHEN 'rolling' THEN generate_series(
                          (DATE ($2 AT TIME ZONE p.tz)
                           - ((DATE ($2 AT TIME ZONE p.tz) - p.anchor_date) % p.rolling_len_days))::timestamp,
                          (DATE ($3 AT TIME ZONE p.tz)
                           - ((DATE ($3 AT TIME ZONE p.tz) - p.anchor_date) % p.rolling_len_days))::timestamp,
                          (p.rolling_len_days || ' days')::interval)
    END AS bucket_start
  FROM params p
),
agg_logs AS (
  SELECT
    b.bucket_start,
    CASE p.period
      WHEN 'daily'   THEN b.bucket_start + INTERVAL '1 day'
      WHEN 'weekly'  THEN b.bucket_start + INTERVAL '7 days'
      WHEN 'monthly' THEN b.bucket_start + INTERVAL '1 month'
      WHEN 'rolling' THEN b.bucket_start + (p.rolling_len_days || ' days')::interval
    END AS bucket_end,
    p.agg,
    p.target_per_period
  FROM buckets b CROSS JOIN params p
),
values_in_bucket AS (
  SELECT
    a.bucket_start,
    CASE p.agg
      WHEN 'sum'     THEN COALESCE(SUM(l.quantity), 0)
      WHEN 'count'   THEN COALESCE(COUNT(*), 0)
      WHEN 'boolean' THEN CASE WHEN COUNT(*) > 0 THEN 1 ELSE 0 END
      -- WHEN 'avg'   THEN COALESCE(AVG(l.quantity), 0)        -- if you add to enum
      -- WHEN 'min'   THEN COALESCE(MIN(l.quantity), 0)
      -- WHEN 'max'   THEN COALESCE(MAX(l.quantity), 0)
      -- WHEN 'last'  THEN COALESCE((ARRAY_AGG(l.quantity ORDER BY l.occurred_at DESC))[1], 0)
    END AS value
  FROM agg_logs a
  JOIN params p ON TRUE
  LEFT JOIN habit_log l
    ON l.habit_id = p.id
   AND l.occurred_at >= a.bucket_start
   AND l.occurred_at <  a.bucket_end
  GROUP BY a.bucket_start, p.agg
)
SELECT
  a.bucket_start,
  a.bucket_end,
  v.value::numeric(12,2) AS value,
  a.target_per_period,
  CASE WHEN a.target_per_period = 0 THEN NULL
       ELSE (v.value / a.target_per_period)
  END AS progress_ratio
FROM agg_logs a
JOIN values_in_bucket v USING (bucket_start)
ORDER BY a.bucket_start;
`
	var rows []BucketRow
	if err := r.db.SelectContext(ctx, &rows, sql, habitID, start, end); err != nil {
		return nil, err
	}
	return rows, nil
}
