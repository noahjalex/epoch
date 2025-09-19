-- dev_schema.sql
\set ON_ERROR_STOP on
\echo '==> Starting dev schema reset'

BEGIN;

-- =========================
-- Extensions
-- =========================
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- for gen_random_uuid()

-- =========================
-- Drops (in dependency order)
-- =========================
-- Drop leaf tables first to avoid relying on CASCADE everywhere
DROP TABLE IF EXISTS public.user_sessions;
DROP TABLE IF EXISTS public.habit_log;
DROP TABLE IF EXISTS public.habit;
DROP TABLE IF EXISTS public.app_user;

-- Drop enum types if they exist (after tables that depend on them are gone)
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'period_type') THEN
    DROP TYPE period_type;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'agg_kind') THEN
    DROP TYPE agg_kind;
  END IF;
END$$;

-- =========================
-- Types
-- =========================
CREATE TYPE period_type AS ENUM ('daily','weekly','monthly','rolling');
CREATE TYPE agg_kind    AS ENUM ('sum','count','boolean');

-- =========================
-- Tables
-- =========================
CREATE TABLE public.app_user (
  id             BIGSERIAL PRIMARY KEY,
  email          CITEXT,
  tz             TEXT NOT NULL DEFAULT 'America/Toronto',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  username       VARCHAR(50)  NOT NULL,
  password_hash  VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX app_user_email_key        ON public.app_user (email);
CREATE UNIQUE INDEX app_user_username_unique  ON public.app_user (username);

CREATE TABLE public.habit (
  id                   BIGSERIAL PRIMARY KEY,
  user_id              BIGINT NOT NULL REFERENCES public.app_user(id) ON DELETE CASCADE,

  name                 TEXT NOT NULL,
  unit_label           TEXT,

  agg                  agg_kind NOT NULL DEFAULT 'sum',

  target_per_period    NUMERIC(12,2) NOT NULL DEFAULT 1,
  per_log_default_qty  NUMERIC(12,2) NOT NULL DEFAULT 1,

  period               period_type NOT NULL DEFAULT 'daily',

  week_start_dow       INT NOT NULL DEFAULT 1 CHECK (week_start_dow BETWEEN 0 AND 6), -- 0..6
  month_anchor_day     INT NOT NULL DEFAULT 1 CHECK (month_anchor_day BETWEEN 1 AND 28),

  rolling_len_days     INT CHECK (rolling_len_days >= 1),
  anchor_date          DATE NOT NULL DEFAULT DATE '1970-01-05', -- Monday baseline

  tz                   TEXT, -- optional override for bucketing

  is_active            BOOLEAN NOT NULL DEFAULT TRUE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX habit_user_idx ON public.habit(user_id);

CREATE TABLE public.habit_log (
  id            BIGSERIAL PRIMARY KEY,
  habit_id      BIGINT NOT NULL REFERENCES public.habit(id) ON DELETE CASCADE,
  occurred_at   TIMESTAMPTZ NOT NULL,      -- store UTC; UI collects in user TZ
  quantity      NUMERIC(12,2) NOT NULL DEFAULT 1 CHECK (quantity >= 0),
  note          TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX habit_log_habit_time_idx ON public.habit_log(habit_id, occurred_at);
CREATE INDEX habit_log_time_idx       ON public.habit_log(occurred_at);

CREATE TABLE public.user_sessions
(
    id UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES public.app_user(id) ON DELETE CASCADE,
    session_token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- If you really need to transfer ownership, uncomment and ensure the role exists:
-- ALTER TABLE public.user_sessions OWNER TO nalexander;

CREATE INDEX idx_user_sessions_token   ON public.user_sessions (session_token);
CREATE INDEX idx_user_sessions_user_id ON public.user_sessions (user_id);
CREATE INDEX idx_user_sessions_expires ON public.user_sessions (expires_at);

COMMIT;

\echo '==> Done. Tables created.'
