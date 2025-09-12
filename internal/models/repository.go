package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var log = logrus.New()

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetUserByID retrieves a user by ID
func (r *Repository) GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(`
		SELECT id, email, tz, created_at 
		FROM users 
		WHERE id = $1
	`, id).Scan(&user.ID, &user.Email, &user.Tz, &user.CreatedAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetHabitsWithDetails retrieves all habits for a user with their current versions and metrics
func (r *Repository) GetHabitsWithDetails(userID int64) ([]HabitWithDetails, error) {
	logger := log.WithFields(map[string]interface{}{
		"method":  "GetHabitsWithDetails",
		"user_id": userID,
	})

	logger.Debug("Starting to fetch habits with details")

	query := `
		SELECT 
			h.id, h.user_id, h.slug, h.is_archived, h.created_at,
			hv.id, hv.habit_id, hv.version, hv.name, hv.description, hv.category, 
			hv.polarity, hv.schedule_rrule, hv.daily_expect_json, hv.active_from, hv.active_to
		FROM habits h
		JOIN habit_versions hv ON h.id = hv.habit_id AND hv.active_to IS NULL
		WHERE h.user_id = $1 AND h.is_archived = false
		ORDER BY h.created_at
	`

	logger.Trace("Executing habits query")
	rows, err := r.db.Query(query, userID)
	if err != nil {
		logger.WithError(err).Error("Failed to execute habits query")
		return nil, err
	}
	defer rows.Close()

	var habits []HabitWithDetails
	for rows.Next() {
		var h HabitWithDetails
		err := rows.Scan(
			&h.Habit.ID, &h.Habit.UserID, &h.Habit.Slug, &h.Habit.IsArchived, &h.Habit.CreatedAt,
			&h.Version.ID, &h.Version.HabitID, &h.Version.Version, &h.Version.Name,
			&h.Version.Description, &h.Version.Category, &h.Version.Polarity,
			&h.Version.ScheduleRrule, &h.Version.DailyExpectJSON, &h.Version.ActiveFrom, &h.Version.ActiveTo,
		)
		if err != nil {
			logger.WithError(err).Error("Failed to scan habit row")
			return nil, err
		}

		habitLogger := logger.WithFields(map[string]interface{}{
			"habit_id":   h.Habit.ID,
			"habit_slug": h.Habit.Slug,
		})

		// Get metrics for this habit
		habitLogger.Debug("Fetching metrics for habit")
		metrics, err := r.GetMetricsForHabit(h.Habit.ID)
		if err != nil {
			habitLogger.WithError(err).Error("Failed to get metrics for habit")
			return nil, err
		}
		h.Metrics = metrics
		habitLogger.WithField("metric_count", len(metrics)).Debug("Successfully fetched metrics")

		// Get targets for this habit
		habitLogger.Debug("Fetching targets for habit")
		targets, err := r.GetTargetsForHabit(h.Habit.ID)
		if err != nil {
			habitLogger.WithError(err).Error("Failed to get targets for habit")
			return nil, err
		}
		h.Targets = targets
		habitLogger.WithField("target_count", len(targets)).Debug("Successfully fetched targets")

		habits = append(habits, h)
	}

	logger.WithField("habit_count", len(habits)).Debug("Successfully fetched all habits with details")
	return habits, nil
}

// GetMetricsForHabit retrieves all metrics for a habit with their current versions
func (r *Repository) GetMetricsForHabit(habitID int64) ([]MetricWithVersion, error) {
	query := `
		SELECT 
			hm.id, hm.habit_id, hm.slug, hm.created_at,
			hmv.id, hmv.metric_id, hmv.version, hmv.name, hmv.metric_kind, 
			hmv.unit_id, hmv.polarity, hmv.agg_kind_default, hmv.min_value, 
			hmv.max_value, hmv.is_required, hmv.active_from, hmv.active_to, hmv.metadata,
			u.id, u.code, u.quantity_kind, u.to_base_factor, u.base_unit_code
		FROM habit_metrics hm
		JOIN habit_metric_versions hmv ON hm.id = hmv.metric_id AND hmv.active_to IS NULL
		LEFT JOIN units u ON hmv.unit_id = u.id
		WHERE hm.habit_id = $1
		ORDER BY hm.created_at
	`

	rows, err := r.db.Query(query, habitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []MetricWithVersion
	for rows.Next() {
		var m MetricWithVersion
		var unitID sql.NullInt64
		var unitCode, unitQuantityKind, unitBaseCode sql.NullString
		var unitToBaseFactor sql.NullFloat64

		err := rows.Scan(
			&m.Metric.ID, &m.Metric.HabitID, &m.Metric.Slug, &m.Metric.CreatedAt,
			&m.Version.ID, &m.Version.MetricID, &m.Version.Version, &m.Version.Name,
			&m.Version.MetricKind, &m.Version.UnitID, &m.Version.Polarity,
			&m.Version.AggKindDefault, &m.Version.MinValue, &m.Version.MaxValue,
			&m.Version.IsRequired, &m.Version.ActiveFrom, &m.Version.ActiveTo, &m.Version.Metadata,
			&unitID, &unitCode, &unitQuantityKind, &unitToBaseFactor, &unitBaseCode,
		)
		if err != nil {
			return nil, err
		}

		if unitID.Valid {
			m.Unit = &Unit{
				ID:           unitID.Int64,
				Code:         unitCode.String,
				QuantityKind: unitQuantityKind.String,
				ToBaseFactor: unitToBaseFactor.Float64,
				BaseUnitCode: unitBaseCode.String,
			}
		}

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// GetTargetsForHabit retrieves all active targets for a habit
func (r *Repository) GetTargetsForHabit(habitID int64) ([]HabitTarget, error) {
	query := `
		SELECT id, habit_id, metric_id, period, agg_kind, target_value, target_bool, active_from, active_to
		FROM habit_targets
		WHERE habit_id = $1 AND (active_to IS NULL OR active_to > NOW())
		ORDER BY active_from
	`

	rows, err := r.db.Query(query, habitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []HabitTarget
	for rows.Next() {
		var t HabitTarget
		err := rows.Scan(
			&t.ID, &t.HabitID, &t.MetricID, &t.Period, &t.AggKind,
			&t.TargetValue, &t.TargetBool, &t.ActiveFrom, &t.ActiveTo,
		)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}

	return targets, nil
}

// CreateHabitLog creates a new habit log with values
func (r *Repository) CreateHabitLog(userID int64, habitSlug string, req LogRequest) (*HabitLog, error) {
	logger := log.WithFields(map[string]interface{}{
		"method":     "CreateHabitLog",
		"user_id":    userID,
		"habit_slug": habitSlug,
	})

	logger.Debug("Starting habit log creation")

	tx, err := r.db.Begin()
	if err != nil {
		logger.WithError(err).Error("Failed to begin transaction")
		return nil, err
	}
	defer tx.Rollback()

	logger.Debug("Transaction started, looking up habit and version")

	// Get habit and current version
	var habitID, habitVersionID int64
	err = tx.QueryRow(`
		SELECT h.id, hv.id
		FROM habits h
		JOIN habit_versions hv ON h.id = hv.habit_id AND hv.active_to IS NULL
		WHERE h.user_id = $1 AND h.slug = $2
	`, userID, habitSlug).Scan(&habitID, &habitVersionID)
	if err != nil {
		logger.WithError(err).Error("Failed to find habit and version")
		return nil, err
	}

	logger.WithFields(map[string]interface{}{
		"habit_id":         habitID,
		"habit_version_id": habitVersionID,
	}).Debug("Found habit and version")

	// Create the log
	var logID int64
	localDay := req.OccurredAt.In(time.UTC).Format("2006-01-02") // Convert to local day

	logger.WithFields(map[string]interface{}{
		"occurred_at": req.OccurredAt,
		"local_day":   localDay,
		"tz":          req.Tz,
		"note":        req.Note,
	}).Debug("Creating habit log entry")

	err = tx.QueryRow(`
		INSERT INTO habit_logs (habit_id, habit_version_id, occurred_at, local_day, tz, note, source)
		VALUES ($1, $2, $3, $4, $5, $6, 'api')
		RETURNING id
	`, habitID, habitVersionID, req.OccurredAt, localDay, req.Tz, req.Note).Scan(&logID)
	if err != nil {
		logger.WithError(err).Error("Failed to create habit log")
		return nil, err
	}

	logger.WithField("log_id", logID).Debug("Created habit log, now creating log values")

	// Create log values
	for i, value := range req.Values {
		valueLogger := logger.WithFields(map[string]interface{}{
			"value_index": i,
			"metric_slug": value.MetricSlug,
			"value_bool":  value.ValueBool,
			"value_num":   value.ValueNum,
		})

		valueLogger.Debug("Processing log value")

		// Get metric and current version
		var metricID, metricVersionID int64
		err = tx.QueryRow(`
			SELECT hm.id, hmv.id
			FROM habit_metrics hm
			JOIN habit_metric_versions hmv ON hm.id = hmv.metric_id AND hmv.active_to IS NULL
			WHERE hm.habit_id = $1 AND hm.slug = $2
		`, habitID, value.MetricSlug).Scan(&metricID, &metricVersionID)
		if err != nil {
			valueLogger.WithError(err).Error("Failed to find metric")
			return nil, fmt.Errorf("metric not found: %s", value.MetricSlug)
		}

		valueLogger.WithFields(map[string]interface{}{
			"metric_id":         metricID,
			"metric_version_id": metricVersionID,
		}).Debug("Found metric, inserting log value")

		// Insert log value
		_, err = tx.Exec(`
			INSERT INTO habit_log_values (log_id, metric_id, metric_version_id, value_bool, value_num)
			VALUES ($1, $2, $3, $4, $5)
		`, logID, metricID, metricVersionID, value.ValueBool, value.ValueNum)
		if err != nil {
			valueLogger.WithError(err).Error("Failed to insert log value")
			return nil, err
		}

		valueLogger.Debug("Successfully inserted log value")
	}

	logger.Debug("All log values created, committing transaction")

	if err = tx.Commit(); err != nil {
		logger.WithError(err).Error("Failed to commit transaction")
		return nil, err
	}

	logger.Info("Successfully created habit log with all values")

	// Return the created log
	return r.GetHabitLogByID(logID)
}

// GetHabitLogByID retrieves a habit log by ID
func (r *Repository) GetHabitLogByID(id int64) (*HabitLog, error) {
	log := &HabitLog{}
	err := r.db.QueryRow(`
		SELECT id, habit_id, habit_version_id, occurred_at, local_day, tz, note, 
		       supersedes_log_id, source, idempotency_key, created_at
		FROM habit_logs
		WHERE id = $1
	`, id).Scan(
		&log.ID, &log.HabitID, &log.HabitVersionID, &log.OccurredAt, &log.LocalDay,
		&log.Tz, &log.Note, &log.SupersedesLogID, &log.Source, &log.IdempotencyKey, &log.CreatedAt,
	)

	if err != nil {
		return nil, err
	}
	return log, nil
}

// GetRecentLogs retrieves recent logs for a user
func (r *Repository) GetRecentLogs(userID int64, limit int) ([]LogWithDetails, error) {
	query := `
		SELECT 
			hl.id, hl.habit_id, hl.habit_version_id, hl.occurred_at, hl.local_day, 
			hl.tz, hl.note, hl.supersedes_log_id, hl.source, hl.idempotency_key, hl.created_at,
			h.id, h.user_id, h.slug, h.is_archived, h.created_at,
			hv.id, hv.habit_id, hv.version, hv.name, hv.description, hv.category, 
			hv.polarity, hv.schedule_rrule, hv.daily_expect_json, hv.active_from, hv.active_to
		FROM habit_logs hl
		JOIN habits h ON hl.habit_id = h.id
		JOIN habit_versions hv ON hl.habit_version_id = hv.id
		WHERE h.user_id = $1
		ORDER BY hl.occurred_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogWithDetails
	for rows.Next() {
		var l LogWithDetails
		err := rows.Scan(
			&l.Log.ID, &l.Log.HabitID, &l.Log.HabitVersionID, &l.Log.OccurredAt, &l.Log.LocalDay,
			&l.Log.Tz, &l.Log.Note, &l.Log.SupersedesLogID, &l.Log.Source, &l.Log.IdempotencyKey, &l.Log.CreatedAt,
			&l.Habit.Habit.ID, &l.Habit.Habit.UserID, &l.Habit.Habit.Slug, &l.Habit.Habit.IsArchived, &l.Habit.Habit.CreatedAt,
			&l.Habit.Version.ID, &l.Habit.Version.HabitID, &l.Habit.Version.Version, &l.Habit.Version.Name,
			&l.Habit.Version.Description, &l.Habit.Version.Category, &l.Habit.Version.Polarity,
			&l.Habit.Version.ScheduleRrule, &l.Habit.Version.DailyExpectJSON, &l.Habit.Version.ActiveFrom, &l.Habit.Version.ActiveTo,
		)
		if err != nil {
			return nil, err
		}

		// Get log values
		values, err := r.GetLogValues(l.Log.ID)
		if err != nil {
			return nil, err
		}
		l.Values = values

		logs = append(logs, l)
	}

	return logs, nil
}

// GetLogValues retrieves log values for a specific log
func (r *Repository) GetLogValues(logID int64) ([]LogValueDetail, error) {
	query := `
		SELECT 
			hlv.id, hlv.log_id, hlv.metric_id, hlv.metric_version_id, 
			hlv.value_bool, hlv.value_num, hlv.metadata,
			hm.id, hm.habit_id, hm.slug, hm.created_at,
			hmv.id, hmv.metric_id, hmv.version, hmv.name, hmv.metric_kind, 
			hmv.unit_id, hmv.polarity, hmv.agg_kind_default, hmv.min_value, 
			hmv.max_value, hmv.is_required, hmv.active_from, hmv.active_to, hmv.metadata,
			u.id, u.code, u.quantity_kind, u.to_base_factor, u.base_unit_code
		FROM habit_log_values hlv
		JOIN habit_metrics hm ON hlv.metric_id = hm.id
		JOIN habit_metric_versions hmv ON hlv.metric_version_id = hmv.id
		LEFT JOIN units u ON hmv.unit_id = u.id
		WHERE hlv.log_id = $1
	`

	rows, err := r.db.Query(query, logID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []LogValueDetail
	for rows.Next() {
		var v LogValueDetail
		var unitID sql.NullInt64
		var unitCode, unitQuantityKind, unitBaseCode sql.NullString
		var unitToBaseFactor sql.NullFloat64

		err := rows.Scan(
			&v.Value.ID, &v.Value.LogID, &v.Value.MetricID, &v.Value.MetricVersionID,
			&v.Value.ValueBool, &v.Value.ValueNum, &v.Value.Metadata,
			&v.Metric.Metric.ID, &v.Metric.Metric.HabitID, &v.Metric.Metric.Slug, &v.Metric.Metric.CreatedAt,
			&v.Metric.Version.ID, &v.Metric.Version.MetricID, &v.Metric.Version.Version, &v.Metric.Version.Name,
			&v.Metric.Version.MetricKind, &v.Metric.Version.UnitID, &v.Metric.Version.Polarity,
			&v.Metric.Version.AggKindDefault, &v.Metric.Version.MinValue, &v.Metric.Version.MaxValue,
			&v.Metric.Version.IsRequired, &v.Metric.Version.ActiveFrom, &v.Metric.Version.ActiveTo, &v.Metric.Version.Metadata,
			&unitID, &unitCode, &unitQuantityKind, &unitToBaseFactor, &unitBaseCode,
		)
		if err != nil {
			return nil, err
		}

		if unitID.Valid {
			v.Metric.Unit = &Unit{
				ID:           unitID.Int64,
				Code:         unitCode.String,
				QuantityKind: unitQuantityKind.String,
				ToBaseFactor: unitToBaseFactor.Float64,
				BaseUnitCode: unitBaseCode.String,
			}
		}

		values = append(values, v)
	}

	return values, nil
}
