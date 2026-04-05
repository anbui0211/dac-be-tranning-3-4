# Makefile Variables

## Variable Assignment Syntax

### `?=` - Conditional Assignment
Assign default value if variable is not already set.

```makefile
DB_HOST ?= localhost
```

**Usage:**
- `make migrate-up` → uses `localhost` (default)
- `make migrate-up DB_HOST=mysql` → overrides to `mysql`

### `:=` - Immediate Assignment
Assigns value immediately when Makefile is parsed.

```makefile
NOW := $(shell date)
```

### `=` - Recursive Assignment
Expands value each time it's used.

```makefile
FILES = $(wildcard *.txt)
```

## Command Line Override

You can override any variable from command line:

```bash
# Override single variable
make migrate-up DB_HOST=localhost

# Override multiple variables
make migrate-up DB_HOST=localhost DB_PORT=3307

# Use flags
make migrate-up VERBOSE=true
```

**Precedence (highest to lowest):**
1. Command line (`make VAR=val`)
2. Environment variables
3. Makefile assignments
