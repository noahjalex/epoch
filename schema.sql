-- dev_schema.sql
\set ON_ERROR_STOP on
\echo '==> Starting dev schema reset and seed...'

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

-- =========================
-- Seed data
-- =========================

-- Users
INSERT INTO public.app_user (email, tz, username, password_hash) VALUES
  ('noah@example.com', 'America/Toronto', 'noah', '$2a$10$9whIqWvGsl/dMDyhe3f.5uLWuvbOgSwCmhTdLtMSM1Ze3erNaMF2.'), -- Password: pass
  ('demo@example.com', 'America/Toronto', 'demo', '$2a$10$9whIqWvGsl/dMDyhe3f.5uLWuvbOgSwCmhTdLtMSM1Ze3erNaMF2.');  -- Password: pass

-- Habits for Noah
-- 1) Workout: 60 minutes daily
INSERT INTO public.habit (user_id, name, unit_label, agg, target_per_period, period, per_log_default_qty)
SELECT id, 'Workout', 'minutes', 'sum', 60, 'daily', 60
FROM public.app_user WHERE email = 'noah@example.com';

-- 2) Learn Dutch words: 3 per week, week starts Monday
INSERT INTO public.habit (user_id, name, unit_label, agg, target_per_period, period, week_start_dow, per_log_default_qty)
SELECT id, 'Learn Dutch words', 'words', 'count', 3, 'weekly', 1, 1
FROM public.app_user WHERE email = 'noah@example.com';

-- 3) Read: 500 pages per month
INSERT INTO public.habit (user_id, name, unit_label, agg, target_per_period, period, per_log_default_qty)
SELECT id, 'Read', 'pages', 'sum', 500, 'monthly', 25
FROM public.app_user WHERE email = 'noah@example.com';

-- 4) Water intake: rolling 7 days, target 14 liters
INSERT INTO public.habit (user_id, name, unit_label, agg, target_per_period, period, rolling_len_days, per_log_default_qty)
SELECT id, 'Water Intake', 'liters', 'sum', 14, 'rolling', 7, 2
FROM public.app_user WHERE email = 'noah@example.com';

-- Grab habit IDs for seeding logs
WITH
  u AS (SELECT id AS user_id FROM public.app_user WHERE email='noah@example.com'),
  h AS (
    SELECT
      (SELECT id FROM public.habit WHERE name='Workout' AND user_id=(SELECT user_id FROM u))           AS workout_id,
      (SELECT id FROM public.habit WHERE name='Learn Dutch words' AND user_id=(SELECT user_id FROM u)) AS dutch_id,
      (SELECT id FROM public.habit WHERE name='Read' AND user_id=(SELECT user_id FROM u))              AS read_id,
      (SELECT id FROM public.habit WHERE name='Water Intake' AND user_id=(SELECT user_id FROM u))      AS water_id
  )
INSERT INTO public.habit_log (habit_id, occurred_at, quantity, note)
SELECT workout_id, TIMESTAMPTZ '2025-09-10 18:00-04', 45, 'Evening run' FROM h UNION ALL
SELECT workout_id, TIMESTAMPTZ '2025-09-11 07:30-04', 65, 'Gym session' FROM h UNION ALL

SELECT dutch_id,   TIMESTAMPTZ '2025-09-09 12:00-04', 1,  'Word #1 (dinsdag)' FROM h UNION ALL
SELECT dutch_id,   TIMESTAMPTZ '2025-09-10 12:00-04', 1,  'Word #2 (woensdag)' FROM h UNION ALL
SELECT dutch_id,   TIMESTAMPTZ '2025-09-12 09:10-04', 1,  'Word #3 (vrijdag)'  FROM h UNION ALL

SELECT read_id,    TIMESTAMPTZ '2025-09-01 20:15-04', 30, 'Chapter 1'      FROM h UNION ALL
SELECT read_id,    TIMESTAMPTZ '2025-09-05 21:00-04', 40, 'Chapters 2-3'   FROM h UNION ALL
SELECT read_id,    TIMESTAMPTZ '2025-09-08 22:05-04', 25, 'Short session'  FROM h UNION ALL
SELECT read_id,    TIMESTAMPTZ '2025-09-12 19:30-04', 60, 'Long read'      FROM h UNION ALL

SELECT water_id,   TIMESTAMPTZ '2025-09-07 08:00-04', 2,  'Morning bottle' FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-08 12:00-04', 1,  'Lunch'          FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-09 09:30-04', 2,  'After run'      FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-10 14:10-04', 2,  'Afternoon'      FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-11 10:05-04', 2,  'Morning'        FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-12 11:45-04', 2,  'Pre-lunch'      FROM h UNION ALL
SELECT water_id,   TIMESTAMPTZ '2025-09-13 09:00-04', 2,  'Morning'        FROM h
;

-- Demo user with a single boolean habit ("Meditate" once per day)
INSERT INTO public.habit (user_id, name, unit_label, agg, target_per_period, period, per_log_default_qty)
SELECT id, 'Meditate', NULL, 'boolean', 1, 'daily', 1
FROM public.app_user WHERE email='demo@example.com';

INSERT INTO public.habit_log (habit_id, occurred_at, quantity, note)
SELECT h.id, TIMESTAMPTZ '2025-09-11 07:00-04', 1, '5-minute breathing'
FROM public.habit h
JOIN public.app_user u ON u.id=h.user_id
WHERE u.email='demo@example.com' AND h.name='Meditate';

COMMIT;

\echo '==> Done. Tables created and sample data inserted.'

