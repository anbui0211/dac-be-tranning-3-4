# Database configuration (default: Docker MySQL)
DB_HOST ?= localhost
DB_PORT ?= 3306
DB_USER ?= appuser
DB_PASSWORD ?= apppassword
DB_NAME ?= appdb

# Migration settings
MIGRATIONS_PATH ?= ./database/migrations
VERBOSE ?= false

# Database connection string
MYSQL_DSN = mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_NAME)?parseTime=true

# Verbose flag
ifeq ($(VERBOSE),true)
	MIGRATE_VERBOSE = -verbose
else
	MIGRATE_VERBOSE =
endif

.PHONY: help migrate-up migrate-down migrate-down-1 migrate-create
.PHONY: migrate-version migrate-force migrate-redo migrate-status

help:
	@echo "Available commands:"
	@echo "  migrate-up             - Apply all pending migrations"
	@echo "  migrate-down           - Rollback all migrations"
	@echo "  migrate-down-1         - Rollback only the most recent migration"
	@echo "  migrate-create NAME=   - Create a new migration file"
	@echo "  migrate-version        - Show current migration version"
	@echo "  migrate-force VERSION= - Force a specific migration version"
	@echo "  migrate-redo           - Rollback and re-apply last migration"
	@echo "  migrate-status         - Show migration status"

migrate-up:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" up $(MIGRATE_VERBOSE)

migrate-down:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down $(MIGRATE_VERBOSE)

migrate-down-1:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down 1 $(MIGRATE_VERBOSE)

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME parameter is required"; \
		echo "Usage: make migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	~/go/bin/migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME)

migrate-version:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" version

migrate-force:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION parameter is required"; \
		echo "Usage: make migrate-force VERSION=N"; \
		exit 1; \
	fi
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" force $(VERSION)

migrate-redo:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down 1 $(MIGRATE_VERBOSE)
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" up 1 $(MIGRATE_VERBOSE)

migrate-status:
	~/go/bin/migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" version
	ls -1 $(MIGRATIONS_PATH)/*.up.sql 2>/dev/null | wc -l | xargs -I {} echo "Total migration files: {}"
