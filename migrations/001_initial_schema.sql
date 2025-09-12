-- Initial schema for habit tracker
-- Based on the provided relational schema

-- Enable citext extension for case-insensitive text
CREATE EXTENSION IF NOT EXISTS citext;

-- Users table
CREATE TABLE IF NOT EXISTS users (
  id               BIGSERIAL PRIMARY KEY,
  email            CITEXT UNIQUE NOT NULL,
  tz               TEXT NOT NULL DEFAULT 'UTC',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Habits table (identity)
CREATE TABLE IF NOT EXISTS habits (
  id               BIGSERIAL PRIMARY KEY,
  user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slug             TEXT NOT NULL,
  is_archived      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, slug)
);

-- Habit versions (versioned definitions)
CREATE TABLE IF NOT EXISTS habit_versions (
  id               BIGSERIAL PRIMARY KEY,
  habit_id         BIGINT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  version          INT NOT NULL,
  name             TEXT NOT NULL,
  description      TEXT,
  category         TEXT,
  polarity         TEXT NOT NULL DEFAULT 'positive',
  schedule_rrule   TEXT,
  daily_expect_json JSONB,
  active_from      TIMESTAMPTZ NOT NULL,
  active_to        TIMESTAMPTZ,
  CONSTRAINT habit_versions_unique UNIQUE (habit_id, version)
);

-- Units for measurements
CREATE TABLE IF NOT EXISTS units (
  id               BIGSERIAL PRIMARY KEY,
  code             TEXT UNIQUE NOT NULL,
  quantity_kind    TEXT NOT NULL,
  to_base_factor   NUMERIC NOT NULL,
  base_unit_code   TEXT NOT NULL
);

-- Habit metrics (identity)
CREATE TABLE IF NOT EXISTS habit_metrics (
  id               BIGSERIAL PRIMARY KEY,
  habit_id         BIGINT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  slug             TEXT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (habit_id, slug)
);

-- Habit metric versions (versioned metric definitions)
CREATE TABLE IF NOT EXISTS habit_metric_versions (
  id               BIGSERIAL PRIMARY KEY,
  metric_id        BIGINT NOT NULL REFERENCES habit_metrics(id) ON DELETE CASCADE,
  version          INT NOT NULL,
  name             TEXT NOT NULL,
  metric_kind      TEXT NOT NULL,
  unit_id          BIGINT REFERENCES units(id),
  polarity         TEXT,
  agg_kind_default TEXT NOT NULL,
  min_value        NUMERIC,
  max_value        NUMERIC,
  is_required      BOOLEAN NOT NULL DEFAULT FALSE,
  active_from      TIMESTAMPTZ NOT NULL,
  active_to        TIMESTAMPTZ,
  metadata         JSONB NOT NULL DEFAULT '{}',
  CONSTRAINT metric_versions_unique UNIQUE (metric_id, version)
);

-- Habit logs (append-only events)
CREATE TABLE IF NOT EXISTS habit_logs (
  id                 BIGSERIAL PRIMARY KEY,
  habit_id           BIGINT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  habit_version_id   BIGINT NOT NULL REFERENCES habit_versions(id),
  occurred_at        TIMESTAMPTZ NOT NULL,
  local_day          DATE NOT NULL,
  tz                 TEXT NOT NULL,
  note               TEXT,
  supersedes_log_id  BIGINT REFERENCES habit_logs(id),
  source             TEXT,
  idempotency_key    TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (habit_id, idempotency_key)
);

-- Habit log values (metric values for each log)
CREATE TABLE IF NOT EXISTS habit_log_values (
  id                 BIGSERIAL PRIMARY KEY,
  log_id             BIGINT NOT NULL REFERENCES habit_logs(id) ON DELETE CASCADE,
  metric_id          BIGINT NOT NULL REFERENCES habit_metrics(id),
  metric_version_id  BIGINT NOT NULL REFERENCES habit_metric_versions(id),
  value_bool         BOOLEAN,
  value_num          NUMERIC,
  metadata           JSONB NOT NULL DEFAULT '{}',
  -- Ensure exactly one value type is set
  CHECK ((value_bool IS NOT NULL)::INT + (value_num IS NOT NULL)::INT = 1)
);

-- Habit targets/goals
CREATE TABLE IF NOT EXISTS habit_targets (
  id                 BIGSERIAL PRIMARY KEY,
  habit_id           BIGINT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  metric_id          BIGINT REFERENCES habit_metrics(id),
  period             TEXT NOT NULL,
  agg_kind           TEXT NOT NULL,
  target_value       NUMERIC,
  target_bool        BOOLEAN,
  active_from        TIMESTAMPTZ NOT NULL,
  active_to          TIMESTAMPTZ
);

-- Derived metrics (formulas)
CREATE TABLE IF NOT EXISTS derived_metrics (
  id                 BIGSERIAL PRIMARY KEY,
  habit_id           BIGINT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  slug               TEXT NOT NULL,
  name               TEXT NOT NULL,
  formula_expr       TEXT NOT NULL,
  result_kind        TEXT NOT NULL,
  unit_id            BIGINT REFERENCES units(id),
  period             TEXT NOT NULL,
  active_from        TIMESTAMPTZ NOT NULL,
  active_to          TIMESTAMPTZ,
  metadata           JSONB NOT NULL DEFAULT '{}',
  UNIQUE (habit_id, slug)
);

-- Derived metric variables
CREATE TABLE IF NOT EXISTS derived_metric_vars (
  id                 BIGSERIAL PRIMARY KEY,
  derived_metric_id  BIGINT NOT NULL REFERENCES derived_metrics(id) ON DELETE CASCADE,
  var_name           TEXT NOT NULL,
  metric_id          BIGINT NOT NULL REFERENCES habit_metrics(id),
  agg_kind           TEXT NOT NULL
);

-- Daily rollups for performance
CREATE TABLE IF NOT EXISTS habit_rollups_daily (
  habit_id           BIGINT NOT NULL,
  local_day          DATE NOT NULL,
  metric_id          BIGINT,
  value_num          NUMERIC,
  value_bool         BOOLEAN,
  computed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (habit_id, local_day, metric_id)
);

-- Metric translations for schema evolution
CREATE TABLE IF NOT EXISTS metric_translations (
  id                 BIGSERIAL PRIMARY KEY,
  from_metric_version_id BIGINT NOT NULL REFERENCES habit_metric_versions(id),
  to_metric_version_id   BIGINT NOT NULL REFERENCES habit_metric_versions(id),
  conversion_kind    TEXT NOT NULL,
  factor             NUMERIC,
  custom_expr        TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_habits_user_id ON habits(user_id);
CREATE INDEX IF NOT EXISTS idx_habit_versions_habit_id ON habit_versions(habit_id);
CREATE INDEX IF NOT EXISTS idx_habit_versions_active ON habit_versions(habit_id, active_to) WHERE active_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_habit_metrics_habit_id ON habit_metrics(habit_id);
CREATE INDEX IF NOT EXISTS idx_habit_metric_versions_metric_id ON habit_metric_versions(metric_id);
CREATE INDEX IF NOT EXISTS idx_habit_metric_versions_active ON habit_metric_versions(metric_id, active_to) WHERE active_to IS NULL;
CREATE INDEX IF NOT EXISTS idx_habit_logs_habit_id ON habit_logs(habit_id);
CREATE INDEX IF NOT EXISTS idx_habit_logs_local_day ON habit_logs(local_day);
CREATE INDEX IF NOT EXISTS idx_habit_log_values_log_id ON habit_log_values(log_id);
CREATE INDEX IF NOT EXISTS idx_habit_rollups_daily_habit_day ON habit_rollups_daily(habit_id, local_day);
