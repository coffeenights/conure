# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Conure is a Kubernetes-native application platform consisting of:
- **API Server** (`cmd/api-server`): RESTful API for application management with authentication, variable storage, and organization management
- **Controller** (`cmd/control`): Kubernetes controller for managing Application custom resources using controller-runtime framework
- **Custom Resources**: Application, Component, Workflow, and Action CRDs defined in `apis/core/v1alpha1/`

## Architecture

The project follows a microservices architecture with two main components:

### API Server (`cmd/api-server/main.go`)
- HTTP server with Gin router
- MongoDB for data persistence
- JWT-based authentication system
- Modular structure: auth, applications, variables, settings, middlewares
- Secret management with both local file storage and Kubernetes secrets

### Kubernetes Controller (`cmd/control/main.go`)
- Kubebuilder-generated controller managing Application CRDs
- Reconciliation logic in `internal/controller/core/`
- Timoni integration for application rendering (`internal/timoni/`)
- Component-based application model

### Key Modules
- `internal/controller/core/`: Controller reconciliation logic
- `internal/k8s/`: Kubernetes client utilities and workload management
- `internal/config/`: Configuration management
- `apis/core/v1alpha1/`: Custom Resource Definitions

## Development Commands

### Build and Test
```bash
# Build the controller binary
make build

# Run all tests with coverage
make test

# Format and vet code
make fmt
make vet

# Generate manifests and code
make manifests
make generate
```

### Running Services

#### API Server
```bash
# Run the API server (default: localhost:8080)
go run ./cmd/api-server/main.go runserver

# Create superuser
go run ./cmd/api-server/main.go createsuperuser -email=admin@example.com

# Reset superuser password
go run ./cmd/api-server/main.go resetsuperuserpassword -email=admin@example.com

# Create secret key for encryption
go run ./cmd/api-server/main.go createsecretkey
```

#### Controller
```bash
# Run controller locally
make run

# Or directly:
go run ./cmd/control/main.go
```

#### Database Setup
```bash
# Start MongoDB with docker-compose
docker-compose up -d mongo
```

### Kubernetes Operations
```bash
# Install CRDs
make install

# Deploy controller to cluster
make deploy

# Uninstall CRDs
make uninstall

# Remove controller deployment
make undeploy
```

### Docker
```bash
# Build controller image
make docker-build

# Push controller image
make docker-push
```

## Environment Configuration

Copy `config.env` to `.env` and configure:
- `DB_URL`: MongoDB connection string
- Database and Redis connection settings
- Authentication secrets

## Testing

The project uses Ginkgo/Gomega for testing:
- Controller tests: `internal/controller/*/suite_test.go`
- API tests: `cmd/api-server/*/test.go`
- Run with: `make test`

## Common Patterns

- Controllers use standard controller-runtime patterns
- API handlers follow Gin conventions
- Authentication via JWT tokens and middleware
- Database operations use MongoDB driver
- Secret storage supports both local files and Kubernetes secrets