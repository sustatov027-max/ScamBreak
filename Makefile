build:
	go build -o bin/FraudService ./FraudService/cmd/service
	go build -o bin/NotificationService ./NotificationService/cmd/service
	go build -o bin/PaymentService ./PaymentService/cmd/service
	go build -o bin/UserService ./UserService/cmd/service

start:
	go run ./FraudService/cmd/service
	go run ./NotificationService/cmd/service
	go run ./PaymentService/cmd/service
	go run ./UserService/cmd/service

# ==========================================
# Database & golang-migrate Configuration
# ==========================================
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_HOST ?= localhost
DB_SSLMODE ?= disable

# UserService DB
USER_DB_PORT ?= 5431
USER_DB_NAME ?= user_db
USER_DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(USER_DB_PORT)/$(USER_DB_NAME)?sslmode=$(DB_SSLMODE)
USER_MIGRATIONS_PATH = UserService/internal/migrations

# PaymentService DB
PAYMENT_DB_PORT ?= 5433
PAYMENT_DB_NAME ?= payment_db
PAYMENT_DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(PAYMENT_DB_PORT)/$(PAYMENT_DB_NAME)?sslmode=$(DB_SSLMODE)
PAYMENT_MIGRATIONS_PATH = PaymentService/internal/migrations

# FraudService DB
FRAUD_DB_PORT ?= 5434
FRAUD_DB_NAME ?= fraud_db
FRAUD_DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(FRAUD_DB_PORT)/$(FRAUD_DB_NAME)?sslmode=$(DB_SSLMODE)
FRAUD_MIGRATIONS_PATH = FraudService/internal/migrations

# NotificationService DB
NOTIFICATION_DB_PORT ?= 5435
NOTIFICATION_DB_NAME ?= notification_db
NOTIFICATION_DB_URL ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(NOTIFICATION_DB_PORT)/$(NOTIFICATION_DB_NAME)?sslmode=$(DB_SSLMODE)
NOTIFICATION_MIGRATIONS_PATH = NotificationService/internal/migrations

# Default rollback step count
N ?= 1

# ==========================================
# Migration Commands (All Services)
# ==========================================
.PHONY: migrate-up migrate-down migrate-version

migrate-up: migrate-up-user migrate-up-payment migrate-up-fraud migrate-up-notification
	@echo "All migrations applied successfully!"

migrate-down: migrate-down-notification migrate-down-fraud migrate-down-payment migrate-down-user
	@echo "All migrations rolled back by $(N) step(s)!"

migrate-version: migrate-version-user migrate-version-payment migrate-version-fraud migrate-version-notification

# ==========================================
# UserService Migrations
# ==========================================
.PHONY: migrate-up-user migrate-down-user migrate-version-user

migrate-up-user:
	@echo "==> Running UserService migrations UP..."
	migrate -path $(USER_MIGRATIONS_PATH) -database "$(USER_DB_URL)" up

migrate-down-user:
	@echo "==> Running UserService migrations DOWN ($(N))..."
	migrate -path $(USER_MIGRATIONS_PATH) -database "$(USER_DB_URL)" down $(N)

migrate-version-user:
	@echo "==> UserService migration version:"
	migrate -path $(USER_MIGRATIONS_PATH) -database "$(USER_DB_URL)" version

# ==========================================
# PaymentService Migrations
# ==========================================
.PHONY: migrate-up-payment migrate-down-payment migrate-version-payment

migrate-up-payment:
	@echo "==> Running PaymentService migrations UP..."
	migrate -path $(PAYMENT_MIGRATIONS_PATH) -database "$(PAYMENT_DB_URL)" up

migrate-down-payment:
	@echo "==> Running PaymentService migrations DOWN ($(N))..."
	migrate -path $(PAYMENT_MIGRATIONS_PATH) -database "$(PAYMENT_DB_URL)" down $(N)

migrate-version-payment:
	@echo "==> PaymentService migration version:"
	migrate -path $(PAYMENT_MIGRATIONS_PATH) -database "$(PAYMENT_DB_URL)" version

# ==========================================
# FraudService Migrations
# ==========================================
.PHONY: migrate-up-fraud migrate-down-fraud migrate-version-fraud

migrate-up-fraud:
	@echo "==> Running FraudService migrations UP..."
	migrate -path $(FRAUD_MIGRATIONS_PATH) -database "$(FRAUD_DB_URL)" up

migrate-down-fraud:
	@echo "==> Running FraudService migrations DOWN ($(N))..."
	migrate -path $(FRAUD_MIGRATIONS_PATH) -database "$(FRAUD_DB_URL)" down $(N)

migrate-version-fraud:
	@echo "==> FraudService migration version:"
	migrate -path $(FRAUD_MIGRATIONS_PATH) -database "$(FRAUD_DB_URL)" version

# ==========================================
# NotificationService Migrations
# ==========================================
.PHONY: migrate-up-notification migrate-down-notification migrate-version-notification

migrate-up-notification:
	@echo "==> Running NotificationService migrations UP..."
	migrate -path $(NOTIFICATION_MIGRATIONS_PATH) -database "$(NOTIFICATION_DB_URL)" up

migrate-down-notification:
	@echo "==> Running NotificationService migrations DOWN ($(N))..."
	migrate -path $(NOTIFICATION_MIGRATIONS_PATH) -database "$(NOTIFICATION_DB_URL)" down $(N)

migrate-version-notification:
	@echo "==> NotificationService migration version:"
	migrate -path $(NOTIFICATION_MIGRATIONS_PATH) -database "$(NOTIFICATION_DB_URL)" version

# ==========================================
# Migration Template / Generator
# Usage: make migrate-create SERVICE=UserService NAME=create_users_table
# ==========================================
.PHONY: migrate-create migrate-force

migrate-create:
ifndef SERVICE
	$(error SERVICE is required. Example: make migrate-create SERVICE=UserService NAME=create_users_table)
endif
ifndef NAME
	$(error NAME is required. Example: make migrate-create SERVICE=UserService NAME=create_users_table)
endif
	@echo "==> Creating migration in $(SERVICE)/internal/migrations..."
	migrate create -ext sql -dir $(SERVICE)/internal/migrations -seq $(NAME)

# Usage: make migrate-force SERVICE=UserService VERSION=1
migrate-force:
ifndef SERVICE
	$(error SERVICE is required. Example: make migrate-force SERVICE=UserService VERSION=1)
endif
ifndef VERSION
	$(error VERSION is required. Example: make migrate-force SERVICE=UserService VERSION=1)
endif
ifeq ($(SERVICE),UserService)
	migrate -path $(USER_MIGRATIONS_PATH) -database "$(USER_DB_URL)" force $(VERSION)
else ifeq ($(SERVICE),PaymentService)
	migrate -path $(PAYMENT_MIGRATIONS_PATH) -database "$(PAYMENT_DB_URL)" force $(VERSION)
else ifeq ($(SERVICE),FraudService)
	migrate -path $(FRAUD_MIGRATIONS_PATH) -database "$(FRAUD_DB_URL)" force $(VERSION)
else ifeq ($(SERVICE),NotificationService)
	migrate -path $(NOTIFICATION_MIGRATIONS_PATH) -database "$(NOTIFICATION_DB_URL)" force $(VERSION)
else
	$(error Unknown SERVICE: $(SERVICE). Must be UserService, PaymentService, FraudService, or NotificationService)
endif


# ==========================================
# RSA Key Generation
# Usage: make generate-keys
# ==========================================
.PHONY: generate-keys

generate-keys:
	go run ./pkg/cmd/keygen -priv certs/jwt_private.pem -pub certs/jwt_public.pem -bits 2048
