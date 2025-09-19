-- =========================
-- Seed data
-- =========================
\set ON_ERROR_STOP on
\echo '==> Starting dev schema seeding'
BEGIN;

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

\echo '==> Done. Seeding db is complete.'
