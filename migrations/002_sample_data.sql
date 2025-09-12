-- Sample data for testing the habit tracker

-- Insert sample units
INSERT INTO units (code, quantity_kind, to_base_factor, base_unit_code) VALUES
('sec', 'time', 1, 'sec'),
('min', 'time', 60, 'sec'),
('hour', 'time', 3600, 'sec'),
('words', 'count', 1, 'words'),
('pages', 'count', 1, 'pages'),
('km', 'distance', 1000, 'm'),
('m', 'distance', 1, 'm'),
('cal', 'energy', 1, 'cal')
ON CONFLICT (code) DO NOTHING;

-- Insert sample user
INSERT INTO users (email, tz) VALUES
('test@example.com', 'America/Toronto')
ON CONFLICT (email) DO NOTHING;

-- Get the user ID for foreign key references
-- In a real app, this would be handled by the application layer

-- Insert sample habits
INSERT INTO habits (user_id, slug) VALUES
(1, 'workout'),
(1, 'brushing'),
(1, 'study'),
(1, 'tv-watching')
ON CONFLICT (user_id, slug) DO NOTHING;

-- Insert habit versions
INSERT INTO habit_versions (habit_id, version, name, description, category, polarity, active_from) VALUES
(1, 1, 'Workout', 'Daily exercise routine', 'health', 'positive', NOW()),
(2, 1, 'Brushing Teeth', 'Brush teeth twice daily', 'health', 'positive', NOW()),
(3, 1, 'Study', 'Language learning session', 'education', 'positive', NOW()),
(4, 1, 'TV Watching', 'Track TV watching time', 'entertainment', 'negative', NOW())
ON CONFLICT (habit_id, version) DO NOTHING;

-- Insert habit metrics
INSERT INTO habit_metrics (habit_id, slug) VALUES
(1, 'duration'),
(1, 'calories'),
(2, 'brushed'),
(3, 'study_time'),
(3, 'words_learned'),
(4, 'tv_minutes')
ON CONFLICT (habit_id, slug) DO NOTHING;

-- Insert habit metric versions
INSERT INTO habit_metric_versions (metric_id, version, name, metric_kind, unit_id, agg_kind_default, active_from) VALUES
-- Workout metrics
(1, 1, 'Duration', 'DURATION', 2, 'SUM', NOW()),  -- minutes
(2, 1, 'Calories', 'QUANTITY', 8, 'SUM', NOW()),  -- calories
-- Brushing metric
(3, 1, 'Brushed', 'BOOLEAN', NULL, 'COUNT_TRUE', NOW()),
-- Study metrics
(4, 1, 'Study Time', 'DURATION', 2, 'SUM', NOW()),  -- minutes
(5, 1, 'Words Learned', 'COUNT', 4, 'SUM', NOW()),  -- words
-- TV watching metric
(6, 1, 'TV Minutes', 'DURATION', 2, 'SUM', NOW())  -- minutes
ON CONFLICT (metric_id, version) DO NOTHING;

-- Insert some sample targets (only if not exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM habit_targets WHERE habit_id = 1 AND metric_id = 1) THEN
        INSERT INTO habit_targets (habit_id, metric_id, period, agg_kind, target_value, active_from) VALUES
        (1, 1, 'DAILY', 'SUM', 1800, NOW()),  -- 30 minutes workout daily (in seconds)
        (2, 3, 'DAILY', 'COUNT_TRUE', 2, NOW()),  -- brush teeth 2 times daily
        (3, 4, 'DAILY', 'SUM', 3600, NOW()),  -- 60 minutes study daily (in seconds)
        (4, 6, 'DAILY', 'SUM', 0, NOW());  -- 0 minutes TV daily (negative habit)
    END IF;
END $$;

-- Insert some sample logs for the past week (only if not exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM habit_logs WHERE habit_id = 1) THEN
        INSERT INTO habit_logs (habit_id, habit_version_id, occurred_at, local_day, tz, source) VALUES
        -- Workout logs
        (1, 1, NOW() - INTERVAL '1 day', (NOW() - INTERVAL '1 day')::date, 'America/Toronto', 'ui'),
        (1, 1, NOW() - INTERVAL '2 days', (NOW() - INTERVAL '2 days')::date, 'America/Toronto', 'ui'),
        (1, 1, NOW() - INTERVAL '3 days', (NOW() - INTERVAL '3 days')::date, 'America/Toronto', 'ui'),
        -- Brushing logs
        (2, 2, NOW() - INTERVAL '1 day' + INTERVAL '8 hours', (NOW() - INTERVAL '1 day')::date, 'America/Toronto', 'ui'),
        (2, 2, NOW() - INTERVAL '1 day' + INTERVAL '20 hours', (NOW() - INTERVAL '1 day')::date, 'America/Toronto', 'ui'),
        (2, 2, NOW() - INTERVAL '2 days' + INTERVAL '8 hours', (NOW() - INTERVAL '2 days')::date, 'America/Toronto', 'ui'),
        -- Study logs
        (3, 3, NOW() - INTERVAL '1 day', (NOW() - INTERVAL '1 day')::date, 'America/Toronto', 'ui'),
        (3, 3, NOW() - INTERVAL '2 days', (NOW() - INTERVAL '2 days')::date, 'America/Toronto', 'ui');

        -- Insert corresponding log values
        INSERT INTO habit_log_values (log_id, metric_id, metric_version_id, value_num) VALUES
        -- Workout values (duration in seconds, calories)
        (1, 1, 1, 2700),  -- 45 minutes
        (1, 2, 2, 350),   -- 350 calories
        (2, 1, 1, 1800),  -- 30 minutes
        (2, 2, 2, 250),   -- 250 calories
        (3, 1, 1, 3600),  -- 60 minutes
        (3, 2, 2, 450),   -- 450 calories
        -- Study values (time in seconds, words)
        (7, 4, 4, 3600),  -- 60 minutes
        (7, 5, 5, 120),   -- 120 words
        (8, 4, 4, 2700),  -- 45 minutes
        (8, 5, 5, 95);    -- 95 words

        -- Insert boolean values for brushing
        INSERT INTO habit_log_values (log_id, metric_id, metric_version_id, value_bool) VALUES
        (4, 3, 3, true),   -- morning brush
        (5, 3, 3, true),   -- evening brush
        (6, 3, 3, true);   -- morning brush day 2
    END IF;
END $$;
