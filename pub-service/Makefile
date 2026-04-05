# Colors for output
COLOR_RESET = \033[0m
COLOR_BOLD = \033[1m
COLOR_GREEN = \033[32m
COLOR_YELLOW = \033[33m
COLOR_RED = \033[31m
COLOR_BLUE = \033[34m

# Database configuration (default: Docker MySQL)
DB_HOST ?= mysql
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
.PHONY: db-connect db-shell db-dump db-logs

help:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Available Migration Commands:$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)migrate-up$(COLOR_RESET)          - Apply all pending migrations"
	@echo "$(COLOR_GREEN)migrate-down$(COLOR_RESET)        - Rollback all migrations (with confirmation)"
	@echo "$(COLOR_GREEN)migrate-down-1$(COLOR_RESET)      - Rollback only the most recent migration (with confirmation)"
	@echo "$(COLOR_GREEN)migrate-create NAME=$(COLOR_RESET)- Create a new migration file (up & down)"
	@echo "$(COLOR_GREEN)migrate-version$(COLOR_RESET)     - Show current migration version"
	@echo "$(COLOR_GREEN)migrate-force VERSION=$(COLOR_RESET)- Force a specific migration version"
	@echo "$(COLOR_GREEN)migrate-redo$(COLOR_RESET)        - Rollback and re-apply last migration"
	@echo "$(COLOR_GREEN)migrate-status$(COLOR_RESET)      - Show migration status"
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Database Commands:$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)db-connect$(COLOR_RESET)         - Connect to MySQL using CLI"
	@echo "$(COLOR_GREEN)db-shell$(COLOR_RESET)           - Connect to MySQL container shell (Docker)"
	@echo "$(COLOR_GREEN)db-dump FILE=$(COLOR_RESET)       - Export database schema and data"
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Options:$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)DB_HOST$(COLOR_RESET)           - Database host (default: mysql)"
	@echo "$(COLOR_YELLOW)VERBOSE=true$(COLOR_RESET)      - Enable verbose output for migrations"
	@echo ""
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Examples:$(COLOR_RESET)"
	@echo "  make migrate-up"
	@echo "  make migrate-up VERBOSE=true"
	@echo "  make migrate-create NAME=add_column_x"
	@echo "  make migrate-force VERSION=2"
	@echo "  make db-connect DB_HOST=localhost"

migrate-up:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Applying migrations...$(COLOR_RESET)"
	@migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" up $(MIGRATE_VERBOSE)
	@echo "$(COLOR_GREEN)✓ Migrations applied successfully$(COLOR_RESET)"

migrate-down:
	@echo "$(COLOR_BOLD)$(COLOR_RED)⚠  WARNING: This will rollback ALL migrations!$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)This operation is irreversible and will result in data loss.$(COLOR_RESET)"
	@read -p "$(COLOR_RED)Are you sure? (yes/no): " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		echo "$(COLOR_BOLD)Rolling back all migrations...$(COLOR_RESET)"; \
		migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down $(MIGRATE_VERBOSE); \
		echo "$(COLOR_GREEN)✓ All migrations rolled back$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)✗ Migration rollback cancelled$(COLOR_RESET)"; \
	fi

migrate-down-1:
	@echo "$(COLOR_BOLD)$(COLOR_YELLOW)Rolling back the most recent migration...$(COLOR_RESET)"
	@read -p "$(COLOR_RED)Are you sure? (yes/no): " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down 1 $(MIGRATE_VERBOSE); \
		echo "$(COLOR_GREEN)✓ Migration rolled back$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)✗ Migration rollback cancelled$(COLOR_RESET)"; \
	fi

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "$(COLOR_RED)✗ Error: NAME parameter is required$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)Usage: make migrate-create NAME=your_migration_name$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Creating migration: $(NAME)$(COLOR_RESET)"
	@migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME)
	@echo "$(COLOR_GREEN)✓ Migration files created:$(COLOR_RESET)"
	@ls -la $(MIGRATIONS_PATH)/*_$(NAME)*.sql 2>/dev/null || true

migrate-version:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Current migration version:$(COLOR_RESET)"
	@migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" version

migrate-force:
	@if [ -z "$(VERSION)" ]; then \
		echo "$(COLOR_RED)✗ Error: VERSION parameter is required$(COLOR_RESET)"; \
		echo "$(COLOR_YELLOW)Usage: make migrate-force VERSION=N$(COLOR_RESET)"; \
		exit 1; \
	fi
	@echo "$(COLOR_BOLD)$(COLOR_YELLOW)Forcing database to version $(VERSION)$(COLOR_RESET)"
	@echo "$(COLOR_RED)⚠  Warning: This should only be used for dirty state recovery!$(COLOR_RESET)"
	@read -p "$(COLOR_RED)Are you sure? (yes/no): " confirm; \
	if [ "$$confirm" = "yes" ]; then \
		migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" force $(VERSION); \
		echo "$(COLOR_GREEN)✓ Database version forced to $(VERSION)$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)✗ Force version cancelled$(COLOR_RESET)"; \
	fi

migrate-redo:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Redoing last migration...$(COLOR_RESET)"
	@migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" down 1 $(MIGRATE_VERBOSE)
	@migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" up 1 $(MIGRATE_VERBOSE)
	@echo "$(COLOR_GREEN)✓ Last migration re-applied$(COLOR_RESET)"

migrate-status:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Migration Status:$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_YELLOW)Database:$(COLOR_RESET) $(DB_USER)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)"
	@echo "$(COLOR_YELLOW)Migrations path:$(COLOR_RESET) $(MIGRATIONS_PATH)"
	@echo ""
	@echo "$(COLOR_YELLOW)Current version:$(COLOR_RESET)"
	@migrate -path $(MIGRATIONS_PATH) -database "$(MYSQL_DSN)" version
	@echo ""
	@echo "$(COLOR_YELLOW)Available migration files:$(COLOR_RESET)"
	@ls -1 $(MIGRATIONS_PATH)/*.up.sql 2>/dev/null | wc -l | xargs -I {} echo "  Total: {} files"

db-connect:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Connecting to MySQL...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Use 'exit' or Ctrl+D to quit$(COLOR_RESET)"
	@mysql -h $(DB_HOST) -P $(DB_PORT) -u $(DB_USER) -p$(DB_PASSWORD) $(DB_NAME)

db-shell:
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Connecting to MySQL container shell...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Use 'exit' or Ctrl+D to quit$(COLOR_RESET)"
	@docker exec -it mysql mysql -u $(DB_USER) -p$(DB_PASSWORD) $(DB_NAME)

db-dump:
	@if [ -z "$(FILE)" ]; then \
		FILE="dump_$(shell date +%Y%m%d_%H%M%S).sql"; \
	fi
	@echo "$(COLOR_BOLD)$(COLOR_BLUE)Exporting database to: $(FILE)$(COLOR_RESET)"
	@mysqldump -h $(DB_HOST) -P $(DB_PORT) -u $(DB_USER) -p$(DB_PASSWORD) $(DB_NAME) > $(FILE)
	@echo "$(COLOR_GREEN)✓ Database exported to $(FILE)$(COLOR_RESET)"
