
# POSTGRES SETUP 

Here are two clean, reliable ways to get a local Postgres called **webby** on your Mac with “simple auth” (no password). Pick Docker (fastest/isolated) or Homebrew (native). I’ll give you drop-in commands and a quick verify.

---

# Option A — Docker (fastest, zero-config trust auth)

**Prereqs:** Docker Desktop installed.

```bash
# 1) Start Postgres 16 in "trust" mode (no password), persisted in a Docker volume
docker run -d --name webby-pg \
  -e POSTGRES_DB=webby \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -p 5432:5432 \
  -v webby_pgdata:/var/lib/postgresql/data \
  postgres:16

# 2) Load your schema+sample data (save your SQL into ./schema.sql first)
docker cp schema.sql webby-pg:/schema.sql
docker exec -i webby-pg psql -U postgres -d webby -f /schema.sql

# 3) Sanity checks
docker exec -it webby-pg psql -U postgres -d webby -c "\dx"              # should show citext
docker exec -it webby-pg psql -U postgres -d webby -c "SELECT count(*) FROM users;"
docker exec -it webby-pg psql -U postgres -d webby -c "SELECT habit_id, local_day, COUNT(*) FROM habit_logs GROUP BY 1,2 ORDER BY 2 DESC;"
```

**Connect from your host tools (psql/GUI):**

* Connection string: `postgres://localhost:5432/webby` (no user/pass needed because container is in trust mode)

Stop / start:

```bash
docker stop webby-pg
docker start webby-pg
```

---

# Option B — Homebrew (native install, trust local connections)

**Prereqs:** Homebrew installed.

```bash
# 1) Install Postgres 16
brew install postgresql@16
brew services start postgresql@16

# 2) Ensure your macOS user is a DB superuser (simplest for local dev)
createuser -s "$(whoami)" || true

# 3) Create the database
createdb webby
```

### Make local connections passwordless (“trust”)

Edit the `pg_hba.conf` for your brew install (path may vary by chip):

* Apple Silicon: `/opt/homebrew/var/postgresql@16/pg_hba.conf`
* Intel: `/usr/local/var/postgresql@16/pg_hba.conf`

Add these **at the top** (before more restrictive lines):

```
# Allow local dev without passwords
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
```

Reload:

```bash
brew services restart postgresql@16
```

### Load your schema + data

Save your SQL as `schema.sql` (exact content you provided). Then:

```bash
psql -d webby -f schema.sql

# sanity checks
psql -d webby -c "\dx"                                 # should list citext
psql -d webby -c "SELECT count(*) FROM users;"
psql -d webby -c "SELECT habit_id, local_day, COUNT(*) FROM habit_logs GROUP BY 1,2 ORDER BY 2 DESC;"
```

**Connect:** `psql postgres://localhost/webby` or any GUI (no password).

---

## Notes on your schema/data (all good, but a few sharp edges)

* **`citext`**: Your script’s `CREATE EXTENSION IF NOT EXISTS citext;` will succeed as DB owner/superuser. Both Docker and Homebrew include `citext` by default.
* **Foreign keys & sample inserts**: You hardcode `user_id = 1` and habit IDs 1..4 implicitly (via `BIGSERIAL`). With a fresh DB, that’s fine. If you re-run the data section, your `ON CONFLICT` guards prevent dupes, but the *FK assumptions* (IDs) rely on the first insert order. For repeatable seeding later, consider capturing IDs via CTEs, e.g. `WITH u AS (INSERT ... RETURNING id)`.
* **Durations**: You sometimes label minutes but store **seconds** (e.g., targets with 1800 = 30 minutes). That’s fine as long as you’re consistent. Your `habit_metric_versions` mark minutes in the `unit_id` (2 = min), yet logs use seconds numerically—be clear in downstream code about whether you normalize to base unit.
* **“Trust” auth** is for local dev *only*. Do not use on a shared machine or network-exposed instance.

---

## One-and-done helper script

If you want a single script for Docker:

```bash
cat > schema.sql <<'SQL'
<PASTE YOUR ENTIRE SQL HERE>
SQL

docker run -d --name webby-pg \
  -e POSTGRES_DB=webby \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -p 5432:5432 \
  -v webby_pgdata:/var/lib/postgresql/data \
  postgres:16

# wait for startup
until docker exec webby-pg pg_isready -U postgres -d webby >/dev/null 2>&1; do sleep 1; done

docker cp schema.sql webby-pg:/schema.sql
docker exec -i webby-pg psql -U postgres -d webby -f /schema.sql
docker exec -it webby-pg psql -U postgres -d webby -c "SELECT email, tz FROM users;"
```

---

## Quick test queries you can run

```sql
-- current habits per user
SELECT u.email, h.id AS habit_id, h.slug, h.is_archived
FROM users u
JOIN habits h ON h.user_id = u.id
ORDER BY h.id;

-- active habit definitions
SELECT hv.habit_id, hv.version, hv.name, hv.category, hv.polarity
FROM habit_versions hv
WHERE hv.active_to IS NULL
ORDER BY hv.habit_id, hv.version;

-- last 7 days of logs (Toronto local day)
SELECT habit_id, local_day, COUNT(*) AS logs
FROM habit_logs
WHERE local_day >= (CURRENT_DATE - INTERVAL '7 days')::date
GROUP BY 1,2
ORDER BY 2 DESC, 1;

-- metrics recorded for a given log
SELECT hl.id AS log_id, hmv.name, COALESCE(hlv.value_num::text, hlv.value_bool::text) AS value
FROM habit_logs hl
JOIN habit_log_values hlv ON hlv.log_id = hl.id
JOIN habit_metric_versions hmv ON hmv.id = hlv.metric_version_id
ORDER BY log_id;
```

---

If you want me to package this as a **Makefile** (targets: `db-up`, `db-seed`, `db-down`) or as a minimal **docker-compose.yml** so you can bring it up with `docker compose up -d`, say the word and I’ll drop both.


----

Got it—let’s nuke it and make a clean, password-auth’d `webby` user + DB. Pick Docker or Homebrew.

---

# Fresh start with Docker (cleanest)

**Why your `webby` user “doesn’t exist”:** the official Postgres image only creates roles/databases from `POSTGRES_USER/POSTGRES_PASSWORD/POSTGRES_DB` on **first init**. If you already had a persistent volume, those envs are ignored on subsequent starts.

## 0) Stop and delete everything from the old container

```bash
docker rm -f webby-pg 2>/dev/null || true
docker volume rm webby_pgdata 2>/dev/null || true   # <- important: remove persisted data
```

## 1) Create a brand-new container with real user/password

```bash
docker run -d --name webby-pg \
  -e POSTGRES_DB=webby \
  -e POSTGRES_USER=webby \
  -e POSTGRES_PASSWORD=devpass \
  -p 5432:5432 \
  -v webby_pgdata:/var/lib/postgresql/data \
  postgres:16
```

## 2) Wait until it’s ready

```bash
until docker exec webby-pg pg_isready -U webby -d webby >/dev/null 2>&1; do sleep 1; done
```

## 3) Enable `citext` (once) and load your schema/data

```bash
# extension (DB owner is webby now, so this works)
docker exec -i webby-pg psql -U webby -d webby -c "CREATE EXTENSION IF NOT EXISTS citext;"

# load schema
docker cp schema.sql webby-pg:/schema.sql
docker exec -i webby-pg psql -U webby -d webby -f /schema.sql
```

## 4) Verify

```bash
docker exec -it webby-pg psql -U webby -d webby -c "\du webby"
docker exec -it webby-pg psql -U webby -d webby -c "SELECT now();"
docker exec -it webby-pg psql -U webby -d webby -c "SELECT COUNT(*) FROM users;"
```

**Go DSN (local, no TLS):**
`postgres://webby:devpass@localhost:5432/webby?sslmode=disable`

---

# Fresh start with Homebrew (native)

You have two choices: quick reset (drop/recreate role+db) or full wipe of the data directory.

## Option 1 — Quick reset (drop/recreate)

```bash
brew services start postgresql@16  # ensure server is running

# Drop existing sessions on webby (if any), then drop DB/role if they exist
psql -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='webby';" 2>/dev/null || true
psql -d postgres -c "DROP DATABASE IF EXISTS webby;"
psql -d postgres -c "DROP ROLE IF EXISTS webby;"

# Recreate role + DB and grant ownership
psql -d postgres -c "CREATE ROLE webby WITH LOGIN PASSWORD 'devpass';"
psql -d postgres -c "CREATE DATABASE webby OWNER webby;"
psql -d webby    -U webby -c "CREATE EXTENSION IF NOT EXISTS citext;"
psql -d webby    -U webby -f schema.sql
```

## Option 2 — Full wipe (new cluster)

> **Danger:** deletes all local PG data.

Apple Silicon path shown; change for Intel (`/usr/local/...`) if needed.

```bash
brew services stop postgresql@16
rm -rf /opt/homebrew/var/postgresql@16

# Re-init the cluster and start
/opt/homebrew/opt/postgresql@16/bin/initdb -D /opt/homebrew/var/postgresql@16
brew services start postgresql@16

# Make the role/db again
psql -d postgres -c "CREATE ROLE webby WITH LOGIN PASSWORD 'devpass';"
psql -d postgres -c "CREATE DATABASE webby OWNER webby;"
psql -d webby    -U webby -c "CREATE EXTENSION IF NOT EXISTS citext;"
psql -d webby    -U webby -f schema.sql
```

**Auth mode:** for local dev, SCRAM password is fine (default). If your client insists, ensure `pg_hba.conf` lines near top include:

```
local   all   all                         scram-sha-256
host    all   all   127.0.0.1/32          scram-sha-256
host    all   all   ::1/128               scram-sha-256
```

Then:

```bash
brew services restart postgresql@16
```

**Go DSN:**
`postgres://webby:devpass@localhost:5432/webby?sslmode=disable`

---

## Sanity check from shell

```bash
psql "postgres://webby:devpass@localhost:5432/webby?sslmode=disable" -c "SELECT current_user, current_database();"
psql "postgres://webby:devpass@localhost:5432/webby?sslmode=disable" -c "\dx"
```

---

## Wire up your Go app

Set env (or `.env`):

```
DB_HOST=localhost
DB_PORT=5432
DB_USER=webby
DB_PASSWORD=devpass
DB_NAME=webby
DB_SSLMODE=disable
```

Make sure your `database.New` builds a DSN that includes `sslmode=disable` for local.

---

If you tell me which route you’re taking (Docker vs Homebrew), I’ll give you a tiny `make db-reset db-seed` or a `docker-compose.yml` so starting fresh is one command instead of six.

