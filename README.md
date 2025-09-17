# Epoch

A habit tracking web application built with Go and PostgreSQL.

## Setup

### Prerequisites
- PostgreSQL 16+
- Go 1.21+

### Database Setup

1. **Create PostgreSQL user and database:**
   ```bash
   # Connect as superuser (usually postgres)
   psql -U postgres
   
   # Create user and database
   CREATE USER epoch WITH PASSWORD 'epoch';
   CREATE DATABASE epoch OWNER epoch;
   \q
   ```

2. **Run schema migrations:**
   ```bash
   # Load the complete schema and sample data
   psql -U epoch -d epoch -f schema.sql
   ```

### Running Locally

1. **Set environment variables:**
   ```bash
   export DB_HOST=localhost
   export DB_PORT=5432
   export DB_USER=epoch
   export DB_PASSWORD=epoch
   export DB_NAME=epoch
   export DB_SSLMODE=disable
   ```

2. **Start the application:**
   ```bash
   go run cmd/web/main.go
   ```

   The app runs on **port 8080** by default. Access it at: http://localhost:8080

   To use a different port:
   ```bash
   go run cmd/web/main.go -port 3000
   ```

### Sample Users

The schema includes two test users:
- **Username:** `noah` **Password:** `pass`
- **Username:** `demo` **Password:** `pass`

## Development

The application uses:
- Go's `net/http` for routing
- PostgreSQL with session-based authentication
- Server-side rendered HTML templates for homepage, login, sign-up and sign-in 
- JavaScript for client interactions

Database connection details are configured via environment variables.
- For local development, use `sslmode=disable`.
