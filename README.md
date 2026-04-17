# Edge Device Management Platform

An integrated platform for managing edge devices, featuring real-time monitoring, data collection, and centralized management capabilities.

## Overview

This platform consists of three main components:

- **Web Frontend** - Vue 3 based dashboard for device management and monitoring
- **Backend Server** - Go-based RESTful API server handling business logic and data persistence
- **Edge Agent** - Go-based lightweight agent deployed on edge devices for data collection and synchronization

## Tech Stack

### Frontend (web/)
- **Framework**: Vue 3 with Composition API
- **State Management**: Pinia
- **Routing**: Vue Router
- **Build Tool**: Vite
- **UI Library**: [To be determined]
- **HTTP Client**: Axios

### Backend Server (server/)
- **Language**: Go 1.21+
- **Web Framework**: [To be determined - e.g., Gin/Echo/Fiber]
- **Database**: PostgreSQL
- **ORM**: GORM or raw SQL
- **Authentication**: JWT
- **Cache**: Redis

### Edge Agent (agent/)
- **Language**: Go 1.21+
- **Communication**: HTTP/gRPC
- **Data Collection**: System metrics, hardware stats
- **Sync Protocol**: WebSocket/HTTP polling

## Project Structure

```
.
├── web/                    # Vue 3 frontend application
│   ├── src/
│   │   ├── api/           # API request modules
│   │   ├── assets/        # Static assets (images, styles)
│   │   ├── components/    # Reusable Vue components
│   │   ├── composables/   # Composition API functions
│   │   ├── directives/    # Custom Vue directives
│   │   ├── router/        # Vue Router configuration
│   │   ├── stores/        # Pinia store modules
│   │   ├── utils/         # Utility functions
│   │   └── views/         # Page components
│   └── public/            # Public static files
├── server/                 # Go backend service
│   ├── cmd/server/        # Application entry point
│   ├── internal/          # Private application code
│   │   ├── api/           # HTTP handlers, middleware, routes
│   │   ├── config/        # Configuration management
│   │   └── domain/        # Business logic (models, repositories, services)
│   └── pkg/               # Public library code
├── agent/                  # Go edge agent
│   ├── cmd/agent/         # Agent entry point
│   ├── internal/          # Private agent code
│   │   ├── collector/     # Data collection modules
│   │   ├── config/        # Configuration management
│   │   ├── publisher/     # Data publishing modules
│   │   └── syncer/        # Synchronization modules
│   └── pkg/               # Public library code
├── .gitignore             # Git ignore rules
├── .env.example           # Environment variables template
└── README.md              # This file
```

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- PostgreSQL 14+
- Redis 7+

### Backend Server Setup

```bash
# Navigate to server directory
cd server

# Copy environment template
cp ../.env.example .env

# Edit .env with your configuration
# ...

# Install dependencies
go mod tidy

# Run database migrations
# [Migration commands to be added]

# Start the server
go run cmd/server/main.go
```

### Frontend Setup

```bash
# Navigate to web directory
cd web

# Install dependencies
npm install

# Copy environment template
cp .env.example .env.local

# Start development server
npm run dev
```

### Edge Agent Setup

```bash
# Navigate to agent directory
cd agent

# Copy environment template
cp ../.env.example .env

# Edit .env with your configuration
# ...

# Build the agent
go build -o edge-agent cmd/agent/main.go

# Run the agent
./edge-agent
```

## Development

### Running Tests

```bash
# Backend tests
cd server && go test ./...

# Frontend tests
cd web && npm run test

# Agent tests
cd agent && go test ./...
```

### Code Style

- **Go**: Follow standard `gofmt` formatting and run `golangci-lint`
- **Vue/TypeScript**: Use ESLint with Vue plugin and Prettier

## Deployment

[Deployment instructions to be added]

## Contributing

[Contribution guidelines to be added]

## License

[License information to be added]
