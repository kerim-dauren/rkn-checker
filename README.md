# RKN Checker

> High-performance microservice for checking URL blocking status against Roskomnadzor registry

[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org/dl/)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)]()

RKN-Checker is a microservice designed to run in Germany that provides real-time URL blocking status checks against the Russian Federal Service for Supervision of Communications, Information Technology, and Mass Media (Roskomnadzor) registry. Built with Go 1.24 and following Clean Architecture principles, it offers both gRPC and REST APIs with sub-millisecond response times.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)  
- [Business Logic](#business-logic)
- [Quick Start](#quick-start)
- [Installation & Deployment](#installation--deployment)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Development](#development)
- [Performance](#performance)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Features

**High Performance**
- Sub-millisecond latency (< 1ms P99)
- >10,000 requests/second throughput
- Thread-safe concurrent operations
- Memory-optimized data structures

**Robust Architecture**
- Clean Architecture with clear layer separation
- Comprehensive error handling and recovery
- Health checks and graceful shutdown
- Race condition protection

**Multiple APIs**
- gRPC API with Protocol Buffers
- REST API with JSON responses
- OpenAPI/Swagger documentation
- Structured logging

**Advanced Blocking Detection**
- Domain-based blocking (exact match)
- Wildcard domain patterns (*.example.com)
- IP address blocking (IPv4/IPv6)
- URL path-based blocking
- HTTPS SNI blocking support

**Real-time Updates**
- Official Roskomnadzor SOAP API integration
- Automatic registry updates every 48 hours
- Retry logic with exponential backoff
- Health checking

**Docker Ready**
- Docker containerization with multi-stage builds
- Comprehensive test coverage (98%)
- Health checks and graceful shutdown

## Architecture

RKN-Checker follows **Clean Architecture** principles with clear separation of concerns across four distinct layers:

```
┌─────────────────────────────────────────────────────────────┐
│                     Delivery Layer                          │
│  ┌─────────────────┐              ┌─────────────────────┐   │
│  │   gRPC Server   │              │   REST Server       │   │
│  │   (Port 9090)   │              │   (Port 80)         │   │
│  └─────────────────┘              └─────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                  Application Layer                          │
│  ┌─────────────────────────────────────────────────────┐   │
│  │         Blocking Service (Use Cases)                │   │
│  │  • CheckURL    • GetStats    • HealthCheck          │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Domain Layer                              │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │   Entities   │  │ Value Objects│  │ Business Rules  │   │
│  │ • Registry   │  │ • URL        │  │ • Blocking Logic│   │
│  │ • Rules      │  │ • Domain     │  │ • Normalization │   │
│  └──────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                Infrastructure Layer                         │
│  ┌─────────────┐ ┌──────────────┐ ┌──────────────────────┐ │
│  │ Memory Store│ │ Registry     │ │ Update Scheduler     │ │
│  │• Bloom Filter│ │ Sources      │ │• Automatic Updates   │ │
│  │• Radix Tree │ │• RKN SOAP API│ │• Retry Logic         │ │
│  │• Thread Safe│ │• Parser      │ │• Health Checking    │ │
│  └─────────────┘ └──────────────┘ └──────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

#### **URL Normalization Engine**
- **Purpose**: Converts URLs to canonical form for consistent matching
- **Features**:
  - Protocol removal (https://, http://, ftp://)
  - Port normalization (removes default ports 80/443)
  - IDN/Punycode handling for international domains
  - Case normalization and trailing slash handling
- **Location**: `internal/domain/services/url_normalizer.go`

#### **Registry Store** 
- **Purpose**: High-performance in-memory storage with optimized data structures
- **Components**:
  - **Bloom Filter**: Fast negative lookups (99.9% accuracy, <1μs)
  - **Radix Tree**: Efficient wildcard domain matching
  - **Hash Maps**: Direct domain and IP lookups
- **Concurrency**: Thread-safe with `sync.RWMutex` protection
- **Location**: `internal/infrastructure/storage/`

#### **Official RKN SOAP API Client**
- **Purpose**: Integrates with official Roskomnadzor web service
- **Protocol**: SOAP 1.1/1.2 with XML over HTTPS
- **Authentication**: Digital signature + client certificates
- **Endpoint**: `https://vigruzki.rkn.gov.ru/services/OperatorRequest/`
- **Features**:
  - Request file and signature management
  - Power of attorney (EMCHD) support
  - Automatic polling for results
  - Health checking via WSDL checks
- **Location**: `internal/infrastructure/registry/official_source.go`

#### **Update Scheduler**
- **Purpose**: Manages automatic registry updates with reliability
- **Features**:
  - Configurable update intervals (default: 48 hours)
  - Exponential backoff retry logic
  - Health checking
  - Graceful error handling
- **Location**: `internal/infrastructure/updater/scheduler.go`

#### **Configuration System**
- **Purpose**: Environment-based configuration with validation
- **Sources**: Environment variables, configuration files
- **Validation**: Comprehensive input validation and error reporting
- **Location**: `internal/infrastructure/config/config.go`

## Business Logic

### URL Blocking Workflow

The service implements a sophisticated multi-stage blocking detection process:

```
┌─────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Request   │───▶│ URL Normalization│───▶│  Bloom Filter   │
│ "example.com"│    │                 │    │  (Quick Check)  │
└─────────────┘    └─────────────────┘    └─────────────────┘
                            │                       │
                            ▼                       ▼
                  ┌─────────────────┐    ┌─────────────────┐
                  │ Canonical URL   │    │ Likely Blocked? │
                  │ "example.com"   │    │  • Yes → Next   │
                  └─────────────────┘    │  • No → Allow   │
                                        └─────────────────┘
                                                 │
                                                 ▼
┌─────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Result    │◀───│ Blocking Rules  │◀───│ Precise Lookup  │
│ • Blocked   │    │   Evaluation    │    │ • Domain Map    │
│ • Reason    │    │                 │    │ • Radix Tree    │
│ • Rule      │    └─────────────────┘    │ • IP Ranges     │
└─────────────┘                          └─────────────────┘
```

### Blocking Rule Types

The service supports multiple types of blocking rules with different priorities:

#### 1. **Domain Blocking** (Priority: High)
```go
// Exact domain match
"example.com" → BLOCKED
"www.example.com" → ALLOWED (unless also listed)
```

#### 2. **Wildcard Domain Blocking** (Priority: Medium)
```go
// Pattern matching with radix tree
"*.example.com" → Blocks all subdomains
"sub.example.com" → BLOCKED
"deep.sub.example.com" → BLOCKED
```

#### 3. **IP Address Blocking** (Priority: High)
```go
// IPv4 and IPv6 support
"192.168.1.100" → BLOCKED
"2001:db8::1" → BLOCKED
```

#### 4. **URL Path Blocking** (Priority: Low)
```go
// Path-specific blocking
"example.com/forbidden" → BLOCKED
"example.com/allowed" → ALLOWED
```

### Registry Data Processing

#### Data Flow Pipeline
1. **Fetch**: SOAP API call to RKN service
2. **Parse**: Multi-format parsing (CSV/ZIP, UTF-8/Windows-1251)
3. **Categorize**: Rule type classification
4. **Normalize**: URL/domain standardization  
5. **Store**: Optimized data structure population
6. **Index**: Bloom filter and radix tree updates

#### Update Strategy
- **Frequency**: Every 48 hours (configurable)
- **Method**: Incremental updates with fallback to full refresh
- **Retry Logic**: Exponential backoff (1s, 2s, 4s, 8s, 16s)
- **Health Checking**: Continuous source availability checks

### Performance Optimizations

#### Memory Efficiency
- **Bloom Filter**: 10M bit array, 7 hash functions
- **String Interning**: Reduces memory fragmentation
- **Copy-on-Write**: Atomic updates without blocking reads

#### CPU Optimization
- **Fast Path**: Bloom filter eliminates 99% of lookups
- **Parallel Processing**: Concurrent request handling
- **Cache-Friendly**: Data structures optimized for CPU cache

#### I/O Optimization
- **Connection Pooling**: Persistent HTTP connections
- **Compression**: gzip encoding for API responses
- **Batch Operations**: Grouped registry updates

## Quick Start

### Prerequisites
- **Go 1.24+**: Required for building from source
- **Docker**: For containerized deployment
- **Authentication Files**: Required for RKN API access
  - Request file (generated through RKN portal)
  - Digital signature file
  - Client certificate (optional, for enhanced security)

### Run with Docker (Recommended)

### Local Development

```bash
# Clone the repository
git clone https://github.com/kerim-dauren/rkn-checker.git
cd rkn-checker

# Install dependencies
go mod download

# Set environment variables
export RKN_REQUEST_FILE_PATH=./certs/request.xml
export RKN_SIGNATURE_FILE_PATH=./certs/signature.bin
export LOG_LEVEL=debug

# Run the service
go run ./cmd/app
```

### Quick API Test

```bash
# Check URL blocking status (REST API)
curl -X POST http://localhost/api/v1/check \
  -H "Content-Type: application/json" \
  -d '{"url": "example.com"}'

# Get registry statistics
curl http://localhost/api/v1/stats

# Health check
curl http://localhost/health
```

## Installation & Deployment

### Docker Deployment

#### Single Container
```bash
# Create directory for authentication files
mkdir -p ./certs

# Copy your RKN authentication files
cp /path/to/your/request.xml ./certs/
cp /path/to/your/signature.bin ./certs/

# Build image locally
docker build -t rkn-checker:latest .

# Run with configuration
docker run -d \
  --name rkn-checker \
  --restart unless-stopped \
  -p 9090:9090 \
  -p 80:80 \
  -e LOG_LEVEL=info \
  -e REGISTRY_UPDATE_INTERVAL=48h \
  -e RKN_REQUEST_FILE_PATH=/certs/request.xml \
  -e RKN_SIGNATURE_FILE_PATH=/certs/signature.bin \
  -v $(pwd)/certs:/certs:ro \
  rkn-checker:latest
```

#### Docker Compose
```yaml
# docker-compose.yml
version: '3.8'
services:
  rkn-checker:
    build:
        context: .
        dockerfile: Dockerfile
    image: rkn-checker:latest
    container_name: rkn-checker
    ports:
      - "9090:9090"
      - "80:80"
    environment:
      - LOG_LEVEL=info
      - REGISTRY_UPDATE_INTERVAL=48h
      - RKN_REQUEST_FILE_PATH=/certs/request.xml
      - RKN_SIGNATURE_FILE_PATH=/certs/signature.bin
    volumes:
      - ./certs:/certs:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### RKN Authentication Setup

#### Obtaining Authentication Files

1. **Register with Roskomnadzor**:
   - Visit https://vigruzki.rkn.gov.ru/auto/
   - Complete the registration process
   - Obtain digital certificates

2. **Generate Request File**:
   ```xml
   <!-- request.xml example -->
   <?xml version="1.0" encoding="UTF-8"?>
   <request>
     <requestTime>2024-01-01T12:00:00</requestTime>
     <operatorName>Your Organization</operatorName>
     <inn>1234567890</inn>
   </request>
   ```

3. **Create Digital Signature**:
   ```bash
   # Sign the request file
   openssl dgst -sha256 -sign private_key.pem request.xml > signature.bin
   ```

#### Security Recommendations

- **File Permissions**: Ensure authentication files have restricted access (600)
- **Secret Management**: Use Kubernetes secrets or Docker secrets for deployment
- **Certificate Rotation**: Regularly update certificates before expiration
- **Network Security**: Deploy behind a firewall or VPN for enhanced security

## Configuration

### Environment Variables

RKN-Checker uses environment variables for configuration. Here are all available options:

#### Core Settings
```bash
# Service Configuration
SERVICE_NAME=rkn-checker              # Service identifier
LOG_LEVEL=info                        # Logging level: debug, info, warn, error
HTTP_PORT=80                          # REST API port
GRPC_PORT=9090                        # gRPC API port

# RKN API Configuration  
RKN_REQUEST_FILE_PATH=/certs/request.xml     # Path to RKN request file
RKN_SIGNATURE_FILE_PATH=/certs/signature.bin # Path to digital signature
RKN_CLIENT_CERT_PATH=/certs/client.crt       # Optional client certificate
RKN_CLIENT_KEY_PATH=/certs/client.key        # Optional client private key
RKN_DUMP_FORMAT_VERSION=2.4                  # RKN dump format version
RKN_POLL_INTERVAL=30s                        # Polling interval for results
RKN_MAX_POLL_ATTEMPTS=20                     # Maximum polling attempts

# Registry Update Configuration
REGISTRY_UPDATE_INTERVAL=48h          # Update frequency
REGISTRY_SOURCE_TIMEOUT=60s           # Source request timeout
REGISTRY_MAX_RETRIES=3                # Maximum retry attempts
REGISTRY_RETRY_BACKOFF=exponential    # Retry strategy

# Performance Tuning
MAX_CONCURRENT_REQUESTS=1000          # Maximum concurrent API requests
BLOOM_FILTER_SIZE=10000000           # Bloom filter bit array size
BLOOM_FILTER_HASH_FUNCS=7            # Number of hash functions
RADIX_TREE_INITIAL_SIZE=100000       # Initial radix tree capacity

# Health Check Configuration
HEALTH_CHECK_INTERVAL=30s            # Health check frequency
HEALTH_CHECK_TIMEOUT=10s             # Health check timeout
HEALTH_CHECK_ENDPOINT=/health        # Health check endpoint path
```

#### HTTP Client Configuration
```bash
# HTTP Client Tuning
HTTP_CLIENT_TIMEOUT=30s              # Default HTTP timeout
HTTP_CLIENT_KEEPALIVE=30s            # Keep-alive duration
HTTP_CLIENT_MAX_IDLE_CONNS=100       # Maximum idle connections
HTTP_CLIENT_MAX_CONNS_PER_HOST=10    # Max connections per host
HTTP_CLIENT_TLS_HANDSHAKE_TIMEOUT=10s # TLS handshake timeout
```

#### gRPC Server Configuration
```bash
# gRPC Server Settings
GRPC_MAX_RECV_MSG_SIZE=4194304       # 4MB max receive message size
GRPC_MAX_SEND_MSG_SIZE=4194304       # 4MB max send message size
GRPC_CONNECTION_TIMEOUT=60s          # Connection timeout
GRPC_KEEPALIVE_TIME=30s              # Keepalive time
GRPC_KEEPALIVE_TIMEOUT=5s            # Keepalive timeout
GRPC_MAX_CONNECTION_IDLE=300s        # Max connection idle time
```

### Configuration File

For complex deployments, you can use a YAML configuration file:

```yaml
# config.yaml
service:
  name: "rkn-checker"
  log_level: "info"
  
server:
  http_port: 80
  grpc_port: 9090
  
rkn:
  request_file_path: "/certs/request.xml"
  signature_file_path: "/certs/signature.bin"
  client_cert_path: "/certs/client.crt"
  client_key_path: "/certs/client.key"
  dump_format_version: "2.4"
  poll_interval: "30s"
  max_poll_attempts: 20
  
registry:
  update_interval: "48h"
  source_timeout: "60s"
  max_retries: 3
  retry_backoff: "exponential"
  
performance:
  max_concurrent_requests: 1000
  bloom_filter_size: 10000000
  bloom_filter_hash_funcs: 7
  radix_tree_initial_size: 100000
```

Load configuration file:
```bash
./rkn-checker --config=/path/to/config.yaml
```

## API Documentation

### REST API

The REST API provides HTTP endpoints for URL checking and service status.

#### Base URL
```
http://localhost:80/api/v1
```

#### Authentication
Currently, the API does not require authentication. Consider implementing API keys or OAuth for secure environments.

#### Endpoints

##### POST /api/v1/check
Check if a URL is blocked by Roskomnadzor.

**Request:**
```json
{
  "url": "example.com",
  "normalize": true
}
```

**Response:**
```json
{
  "url": "example.com",
  "normalized_url": "example.com",
  "blocked": true,
  "blocking_type": "domain",
  "rule_id": "12345",
  "reason": "Blocked by exact domain match",
  "checked_at": "2024-01-01T12:00:00Z",
  "registry_version": "2024.01.01.001"
}
```

**Status Codes:**
- `200 OK`: Successful check
- `400 Bad Request`: Invalid URL format
- `422 Unprocessable Entity`: URL normalization failed
- `500 Internal Server Error`: Service error

##### GET /api/v1/stats
Get registry statistics and service information.

**Response:**
```json
{
  "registry": {
    "total_entries": 1500000,
    "domain_entries": 800000,
    "wildcard_entries": 300000,
    "ip_entries": 250000,
    "url_entries": 150000,
    "last_update": "2024-01-01T10:00:00Z",
    "next_update": "2024-01-03T10:00:00Z",
    "update_interval": "48h"
  },
  "performance": {
    "bloom_filter_size": 10000000,
    "bloom_filter_false_positive_rate": 0.001,
    "radix_tree_nodes": 45000,
    "memory_usage_mb": 450
  },
  "service": {
    "version": "1.0.0",
    "uptime": "72h30m15s",
    "requests_processed": 2500000,
    "avg_response_time_ms": 0.8
  }
}
```

##### GET /health
Health check endpoint for load balancers and health checks.

**Response (Healthy):**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "checks": {
    "registry_source": "healthy",
    "memory_store": "healthy",
    "last_update": "2024-01-01T10:00:00Z"
  }
}
```

**Response (Unhealthy):**
```json
{
  "status": "unhealthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "checks": {
    "registry_source": "unhealthy",
    "memory_store": "healthy",
    "last_update": "2024-01-01T08:00:00Z"
  },
  "errors": [
    "RKN API service unavailable for 2 hours"
  ]
}
```

#### Error Handling

All API responses include consistent error formatting:

```json
{
  "error": {
    "code": "INVALID_URL",
    "message": "The provided URL format is invalid",
    "details": {
      "url": "not-a-valid-url",
      "validation_errors": ["missing scheme", "invalid domain"]
    }
  }
}
```

#### Rate Limiting

The API supports configurable rate limiting:
- Default: 1000 requests per second per IP
- Configurable via `MAX_CONCURRENT_REQUESTS` environment variable
- Rate limit headers included in responses:
  - `X-RateLimit-Limit`: Request limit
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Reset timestamp

### gRPC API

The gRPC API provides high-performance binary communication using Protocol Buffers.

#### Service Definition

```protobuf
// blocking_service.proto
syntax = "proto3";

package blocking;

option go_package = "github.com/kerim-dauren/rkn-checker/proto";

service BlockingService {
  rpc CheckURL(CheckURLRequest) returns (CheckURLResponse);
  rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message CheckURLRequest {
  string url = 1;
  bool normalize = 2;
}

message CheckURLResponse {
  string url = 1;
  string normalized_url = 2;
  bool blocked = 3;
  string blocking_type = 4;
  string rule_id = 5;
  string reason = 6;
  int64 checked_at = 7;
  string registry_version = 8;
}
```

#### Client Examples

**Go Client:**
```go
package main

import (
    "context"
    "log"
    
    "google.golang.org/grpc"
    pb "github.com/kerim-dauren/rkn-checker/proto"
)

func main() {
    conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    client := pb.NewBlockingServiceClient(conn)
    
    resp, err := client.CheckURL(context.Background(), &pb.CheckURLRequest{
        Url: "example.com",
        Normalize: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("URL %s is blocked: %v", resp.Url, resp.Blocked)
}
```

#### Performance Considerations

- **Connection Pooling**: Reuse gRPC connections for multiple requests
- **Keepalive Settings**: Configure appropriate keepalive timeouts
- **Message Size Limits**: Default 4MB max message size
- **Compression**: Enable gzip compression for large responses

## Development

### Local Development Setup

#### Prerequisites
```bash
# Go 1.24+
go version

# Development tools
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Testing tools  
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install gotest.tools/gotestsum@latest
```

#### Development Commands
```bash
# Run tests with coverage
go test ./... -race -cover

# Generate code coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Lint code
golangci-lint run ./...

# Format code
go fmt ./...
go mod tidy

# Generate protocol buffers
protoc --go_out=. --go-grpc_out=. proto/*.proto

# Build development binary
go build -race -o rkn-checker-dev ./cmd/app

# Run with test configuration
RKN_REQUEST_FILE_PATH=./test/fixtures/request.xml \
RKN_SIGNATURE_FILE_PATH=./test/fixtures/signature.bin \
LOG_LEVEL=debug \
./rkn-checker-dev
```

### Testing Strategy

#### Unit Tests
- **Coverage Target**: >95%
- **Test Structure**: Table-driven tests with extensive edge cases
- **Mocking**: Interface-based mocking for external dependencies
- **Race Detection**: All tests run with `-race` flag

#### Integration Tests
```bash
# Run integration tests with real RKN API
go test -tags=integration ./test/integration/... -v

# Run tests with mock services
go test ./test/integration/... -mock-services
```

#### Performance Tests
```bash
# Benchmark core functions
go test -bench=. -benchmem ./internal/...

# Load testing with artillery
artillery quick --count 100 --num 1000 http://localhost/api/v1/check
```

#### Test Data Management
```bash
# Generate test registry data
go run ./cmd/testdata-generator --entries=10000 --output=./test/fixtures/

# Validate test fixtures
go test ./test/fixtures/... -validate-only
```

## Performance

### Benchmarks

Current performance metrics on standard hardware:

#### Throughput
- **REST API**: 12,000 requests/second (single instance)
- **gRPC API**: 15,000 requests/second (single instance)
- **Concurrent Requests**: 50,000+ simultaneous connections

#### Latency (P99)
- **URL Check**: <1ms (bloom filter hit)
- **URL Check**: <5ms (full registry lookup)
- **Registry Update**: <30s (1M entries)

#### Memory Usage
- **Base Service**: ~50MB
- **1M Registry Entries**: ~200MB additional
- **10M Registry Entries**: ~450MB additional

#### CPU Usage
- **Idle**: <1% CPU usage
- **Under Load**: 15-25% CPU (1000 RPS)

### Performance Tuning

#### Memory Optimization
```go
// Tuning parameters in config
performance:
  bloom_filter_size: 10000000        # Larger = fewer false positives
  bloom_filter_hash_funcs: 7         # Optimal for 0.1% false positive rate  
  radix_tree_initial_size: 100000    # Pre-allocate for known data size
  max_concurrent_requests: 1000       # Balance memory vs. throughput
```

#### CPU Optimization
```bash
# Runtime tuning
export GOMAXPROCS=4                   # Match container CPU limit
export GOGC=100                       # Adjust GC pressure
```

#### I/O Optimization
```bash
# Network tuning
echo 'net.ipv4.tcp_tw_reuse = 1' >> /etc/sysctl.conf
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
```

## Security

### Security Considerations

#### Authentication & Authorization
- **API Security**: Consider implementing API keys for secure environments
- **Certificate Management**: Secure storage of RKN certificates
- **Access Control**: Network-level restrictions for sensitive endpoints

#### Data Protection
- **In-Transit**: All RKN API communications use HTTPS/TLS
- **At-Rest**: Configuration files should have restricted permissions
- **Logging**: Avoid logging sensitive authentication data

#### Container Security
```dockerfile
# Security-hardened Dockerfile
FROM golang:1.24-alpine AS builder
RUN apk add --no-cache ca-certificates
# ... build steps ...

FROM scratch AS runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/rkn-checker /rkn-checker
USER 65534:65534
ENTRYPOINT ["/rkn-checker"]
```

#### Network Security
- Deploy behind reverse proxy (nginx/Traefik)
- Use network policies in Kubernetes
- Implement rate limiting and DDoS protection

## Troubleshooting

### Common Issues

#### Service Won't Start
**Symptoms**: Service fails to start or exits immediately

**Potential Causes:**
1. **Missing authentication files**
   ```bash
   # Check file existence and permissions
   ls -la /certs/request.xml /certs/signature.bin
   # Should be readable by service user (600 or 644)
   ```

2. **Port conflicts**
   ```bash
   # Check if ports are in use
   netstat -tlnp | grep -E "(80|9090)"
   # Change ports via environment variables if needed
   ```

3. **Invalid configuration**
   ```bash
   # Test configuration parsing
   ./rkn-checker --config=/path/to/config.yaml --validate-only
   ```

#### High Memory Usage
**Symptoms**: Service consumes excessive memory

**Solutions:**
1. **Tune bloom filter size**
   ```bash
   export BLOOM_FILTER_SIZE=5000000  # Reduce if memory constrained
   ```

2. **Check GC behavior**
   ```bash
   export GODEBUG=gctrace=1  # Enable GC logging
   ```

3. **Set memory limits**
   ```bash
   # Docker memory limit
   docker run --memory=512m rkn-checker:latest
   ```

#### Registry Updates Failing
**Symptoms**: Registry data becomes stale, health checks fail

**Troubleshooting Steps:**
1. **Check RKN API connectivity**
   ```bash
   curl -I https://vigruzki.rkn.gov.ru/services/OperatorRequest/
   ```

2. **Verify authentication files**
   ```bash
   # Test file content (should be XML and binary respectively)
   file /certs/request.xml /certs/signature.bin
   head -n5 /certs/request.xml  # Should show XML header
   ```

3. **Check service logs**
   ```bash
   # Look for specific error messages
   docker logs rkn-checker | grep -i "rkn\|registry\|auth"
   ```

4. **Manual API test**
   ```bash
   # Test authentication with RKN service
   curl -X POST \
     -H "Content-Type: text/xml" \
     --cert /certs/client.crt \
     --key /certs/client.key \
     -d @/certs/request.xml \
     https://vigruzki.rkn.gov.ru/services/OperatorRequest/
   ```

#### High Error Rates
**Symptoms**: API returning 500 errors, failing health checks

**Investigation Steps:**
1. **Check resource limits**
   ```bash
   # CPU and memory usage
   docker stats rkn-checker
   ```

2. **Review error logs**
   ```bash
   # Filter for error-level messages
   docker logs rkn-checker | grep '"level":"error"'
   ```

3. **Test individual components**
   ```bash
   # Health check endpoint
   curl -v http://localhost/health
   
   # Simple URL check
   curl -X POST http://localhost/api/v1/check \
     -H "Content-Type: application/json" \
     -d '{"url": "google.com"}'
   ```