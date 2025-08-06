# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TimeLocker Backend is a Go-based blockchain asset management platform built with the Gin framework. It provides multi-chain support for timelock contracts, asset management via Covalent API, and wallet-based authentication.

## Development Commands

### Running the Application
```bash
# Start the server (development mode)
go run cmd/server/main.go

# Build the application
go build -o timelocker-backend cmd/server/main.go
```

### Database Management
```bash
# Database will auto-migrate on startup via pkg/database/migrations/init_tables.go
# No manual migration commands needed - the system handles this automatically
```

### Dependencies
```bash
# Install/update dependencies
go mod download
go mod tidy
```

### Testing
```bash
# Run tests (if test files exist)
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Development Tools
```bash
# Generate Swagger documentation (if needed)
go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/server/main.go
```

## Configuration

The application uses `config.yaml` for configuration. Key settings include:

- **Server**: Port (default 8080), mode (debug/release/test)
- **Database**: PostgreSQL connection details  
- **Redis**: Cache configuration
- **Covalent API**: For blockchain asset data
- **RPC**: Alchemy/Infura API keys for blockchain interaction
- **JWT**: Authentication token settings

Environment variables can override config values using the `TIMELOCKER_` prefix.

## Architecture Overview

### Directory Structure
- `cmd/server/` - Application entry point with main.go
- `internal/` - Private application code
  - `api/` - HTTP handlers organized by domain (auth, asset, chain, etc.)
  - `service/` - Business logic layer
  - `repository/` - Data access layer (GORM-based)
  - `config/` - Configuration management
  - `types/` - Domain models and data structures
  - `middleware/` - HTTP middleware (auth, CORS)
- `pkg/` - Reusable packages
  - `database/` - Database connections and migrations
  - `logger/` - Structured logging (Zap)
  - `utils/` - JWT management and utilities

### Key Components

**Authentication**: Wallet-based authentication using cryptographic signatures. No traditional username/password - users authenticate with their wallet address and signature.

**Multi-chain Support**: Supports 15+ blockchains (Ethereum, BSC, Polygon, Arbitrum, etc.) with both mainnet and testnet configurations stored in the `support_chains` table.

**Asset Management**: Integrates with Covalent API to fetch real-time token balances, prices, and 24h price changes. Assets are cached in Redis and PostgreSQL.

**Timelock Contracts**: Supports both Compound and OpenZeppelin timelock standards with contract deployment and management capabilities.

**Database Design**: Uses PostgreSQL with GORM. Auto-migration system handles schema updates safely without data loss via `pkg/database/migrations/init_tables.go`.

## Development Workflow

1. **Database Setup**: The system auto-migrates on startup. Initial data includes supported chains, shared ABIs, and default sponsors.

2. **API Documentation**: Swagger docs available at `/swagger/index.html` when server is running.

3. **Health Checks**: 
   - `/health` - Basic service health
   - `/api/v1/system/rpc-status` - RPC configuration status

4. **Authentication Flow**: 
   - Client connects wallet via `/api/v1/auth/wallet-connect`
   - Provides wallet address, signature, and chain ID
   - Server validates signature and returns JWT tokens

5. **Asset Management**:
   - `/api/v1/assets` - Get user assets (auto-refresh)
   - `/api/v1/assets/refresh` - Manual refresh from Covalent API

## Important Notes

- **No Traditional Tests**: This codebase doesn't appear to have unit tests. Consider adding them in a `*_test.go` pattern.
- **Security**: Never commit API keys. Use environment variables for sensitive configuration.
- **Database Migrations**: The migration system is version-controlled and safe. Don't bypass it with manual schema changes.
- **Multi-chain**: When adding new chains, update the migration data in `insertSupportedChains()`.
- **Caching**: Asset data is cached in Redis with configurable expiry (default 5 minutes).

## API Key Requirements

The application requires:
- **Covalent API Key**: For blockchain asset data
- **Alchemy or Infura API Key**: For RPC calls to blockchains
- **SMTP credentials**: For email notifications (optional)

## Server Startup

The server starts on port 8080 (configurable) and provides:
- REST API at `/api/v1/*`
- Swagger documentation at `/swagger/index.html`
- Health check at `/health`
- CORS enabled for all origins in development mode