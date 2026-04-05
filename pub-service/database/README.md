# Migration Setup Guide

This directory contains database migrations for the pub-service using golang-migrate.

## Prerequisites

1. **Install golang-migrate CLI** (one-time setup):
   ```bash
   go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
   ```

2. **Start MySQL container**:
   ```bash
   docker-compose up -d mysql
   ```

## Using Makefile

The Makefile provides convenient commands for managing migrations.

### Show All Commands
```bash
make help
```

### Apply Migrations
```bash
# Apply all pending migrations
make migrate-up

# With verbose output
make migrate-up VERBOSE=true
```

### Rollback Migrations
```bash
# Rollback all migrations (requires confirmation)
make migrate-down

# Rollback only the most recent migration (requires confirmation)
make migrate-down-1
```

### Create New Migration
```bash
make migrate-create NAME=add_column_x
```

### Check Migration Status
```bash
# Show current version
make migrate-version

# Show detailed status
make migrate-status
```

### Force Version (Dirty State Recovery)
```bash
make migrate-force VERSION=2
```

### Reapply Last Migration
```bash
make migrate-redo
```

### Database Commands
```bash
# Connect to MySQL using CLI
make db-connect

# Connect to MySQL container shell (Docker)
make db-shell

# Export database
make db-dump FILE=backup.sql
```

### Using Local MySQL Instead of Docker
```bash
# Override DB_HOST for local development
make migrate-up DB_HOST=localhost
make db-connect DB_HOST=localhost
```

## Migration Files

### 000001_init.sql
- Creates `contents` table
- Creates `message_schedules` table
- Seeds 5 content records
- Seeds 5 message schedule records

## Environment Variables

The following environment variables are used (can be overridden):

- `DB_HOST` (default: `mysql`)
- `DB_PORT` (default: `3306`)
- `DB_USER` (default: `appuser`)
- `DB_PASSWORD` (default: `apppassword`)
- `DB_NAME` (default: `appdb`)

## GORM Integration

The `pkg/db/mysql.go` file provides GORM integration:

```go
import "pub-service/pkg/db"

// Initialize MySQL provider
mysqlProvider, err := db.NewMySQLProvider(ctx)
if err != nil {
    log.Fatalf("Failed to initialize MySQL provider: %v", err)
}
defer mysqlProvider.Close()

// Get GORM DB instance
db := mysqlProvider.GetDB()

// Use GORM for CRUD operations
var contents []model.Content
db.Find(&contents)
```

## Testing Migration

After applying migrations, verify the tables and data:

```bash
# Connect to MySQL
make db-shell

# Check tables
SHOW TABLES;

# Check contents
SELECT * FROM contents;

# Check message_schedules
SELECT * FROM message_schedules;

# Exit
exit
```

## Troubleshooting

### Dirty Database State
If a migration fails and the database is marked as "dirty":

1. Check the current version: `make migrate-version`
2. Identify the correct version (usually one less than shown)
3. Force the correct version: `make migrate-force VERSION=N`
4. Fix the migration issue
5. Reapply: `make migrate-up`

### Migration Already Applied
If you see `ErrNoChange` or similar errors, the migration has already been applied. This is normal and safe.

### Connection Issues
If you can't connect to MySQL:

1. Check if MySQL is running: `docker ps | grep mysql`
2. Check MySQL logs: `docker logs mysql`
3. Verify network: `docker network ls` and `docker network inspect be-training-3-4_app-network`

## Best Practices

1. **Always test migrations locally** before deploying
2. **Use descriptive names** for migration files
3. **Make migrations reversible** - ensure down.sql works
4. **Keep migrations small** - one change per migration
5. **Backup before rollback** - use `make db-dump` before running `make migrate-down`
6. **Version control** - commit migration files to git
