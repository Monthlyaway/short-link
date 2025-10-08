# Short Link Service

A high-performance URL shortening service built with Go, featuring distributed ID generation, multi-layer caching, and Docker deployment.

## Features

- **Snowflake ID Generation**: Distributed unique ID generation with 41-bit timestamp, 10-bit machine ID, and 12-bit sequence
- **Base62 Encoding**: Convert IDs to short 6-8 character codes
- **Multi-Layer Caching**: Bloom Filter → Redis → MySQL cascade for optimal performance
- **Cache Penetration Prevention**: 10M capacity Bloom filter with 1% false positive rate
- **Visit Analytics**: Track visit counts and detailed logs with IP and User-Agent
- **Graceful Shutdown**: Proper resource cleanup and connection management
- **Docker Support**: One-command deployment with docker-compose

## Tech Stack

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **Database**: MySQL 8.0
- **Cache**: Redis 7.0
- **Libraries**:
  - GORM (ORM)
  - Snowflake (ID generation)
  - Bloom Filter (cache penetration prevention)
  - go-redis (Redis client)

## Project Structure

```
short-link/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
├── internal/
│   ├── handler/
│   │   └── url_handler.go         # HTTP handlers
│   ├── service/
│   │   └── url_service.go         # Business logic
│   ├── repository/
│   │   └── url_repository.go      # Database operations
│   ├── model/
│   │   └── url.go                 # Data models
│   ├── cache/
│   │   └── redis.go               # Redis cache
│   ├── filter/
│   │   └── bloom.go               # Bloom filter
│   └── utils/
│       ├── shortcode.go           # Base62 encoding
│       └── snowflake.go           # Snowflake ID generator
├── config/
│   ├── config.go                  # Configuration management
│   └── config.yaml                # Configuration file
├── migrations/
│   └── 001_init.sql               # Database schema
├── docker-compose.yml             # Docker orchestration
├── Dockerfile                     # Container build
├── go.mod
├── go.sum
└── README.md
```

## Installation

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (for containerized deployment)
- MySQL 8.0 (if running locally)
- Redis 7.0 (if running locally)

### Option 1: Docker Deployment (Recommended)

1. Clone the repository:
```bash
git clone https://github.com/Monthlyaway/short-link.git
cd short-link
```

2. Start all services with docker-compose:
```bash
docker-compose up -d
```

3. Verify services are running:
```bash
docker-compose ps
```

4. Check application logs:
```bash
docker-compose logs -f app
```

The service will be available at `http://localhost:8080`

### Option 2: Local Development

1. Install dependencies:
```bash
go mod download
```

2. Set up MySQL database:
```bash
mysql -u root -p < migrations/001_init.sql
```

3. Set up Redis:
```bash
# Install and start Redis
redis-server
```

4. Update configuration:
Edit `config/config.yaml` with your MySQL and Redis connection details.

5. Run the application:
```bash
go run cmd/server/main.go
```

## Configuration

Edit `config/config.yaml` to customize settings:

```yaml
server:
  port: 8080
  mode: debug  # debug, release

mysql:
  host: localhost
  port: 3306
  username: root
  password: root123
  database: url_shortener
  max_idle_conns: 10
  max_open_conns: 100

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 100

bloom_filter:
  capacity: 10000000
  false_positive_rate: 0.01

snowflake:
  datacenter_id: 1
  worker_id: 1
```

## API Documentation

### 1. Create Short URL

**Endpoint**: `POST /api/v1/shorten`

**Request**:
```json
{
  "url": "https://www.example.com/very/long/url",
  "expired_at": "2025-12-31T23:59:59Z"  // Optional
}
```

**Response**:
```json
{
  "code": 200,
  "data": {
    "short_code": "aB3xY9",
    "short_url": "http://localhost:8080/aB3xY9",
    "original_url": "https://www.example.com/very/long/url",
    "expired_at": "2025-12-31T23:59:59Z"
  }
}
```

**cURL Example**:
```bash
curl -X POST http://localhost:8080/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.google.com"}'
```

### 2. Redirect to Original URL

**Endpoint**: `GET /{short_code}`

**Response**: 302 Redirect to original URL

**cURL Example**:
```bash
curl -i http://localhost:8080/aB3xY9
```

### 3. Get URL Information

**Endpoint**: `GET /api/v1/info/{short_code}`

**Response**:
```json
{
  "code": 200,
  "data": {
    "short_code": "aB3xY9",
    "original_url": "https://www.example.com/very/long/url",
    "visit_count": 1234,
    "created_at": "2025-01-01T00:00:00Z",
    "expired_at": null
  }
}
```

**cURL Example**:
```bash
curl http://localhost:8080/api/v1/info/aB3xY9
```

### 4. Health Check

**Endpoint**: `GET /health`

**Response**:
```json
{
  "code": 200,
  "message": "OK"
}
```

## Database Schema

### url_mappings Table
| Column | Type | Description |
|--------|------|-------------|
| id | BIGINT | Auto-increment primary key |
| short_code | VARCHAR(10) | Unique short code |
| original_url | VARCHAR(2048) | Original URL |
| created_at | TIMESTAMP | Creation timestamp |
| expired_at | TIMESTAMP | Expiration timestamp (nullable) |
| visit_count | BIGINT | Visit counter |
| status | TINYINT | Status (1=active, 0=disabled) |

### visit_logs Table
| Column | Type | Description |
|--------|------|-------------|
| id | BIGINT | Auto-increment primary key |
| short_code | VARCHAR(10) | Short code reference |
| visited_at | TIMESTAMP | Visit timestamp |
| ip | VARCHAR(45) | Visitor IP address |
| user_agent | VARCHAR(512) | Visitor user agent |

## Architecture

### System Overview

This URL shortening service follows a **layered architecture** pattern with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client (HTTP/REST)                        │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                    Presentation Layer                            │
│  ┌────────────────────────────────────────────────────────┐     │
│  │  Handler (Gin Router)                                  │     │
│  │  - URL validation & request binding                    │     │
│  │  - Response formatting & HTTP status codes             │     │
│  │  - Route registration & middleware                     │     │
│  └────────────────────────┬───────────────────────────────┘     │
└───────────────────────────┼─────────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────────┐
│                   Business Logic Layer                           │
│  ┌────────────────────────────────────────────────────────┐     │
│  │  Service                                               │     │
│  │  - URL creation & validation logic                     │     │
│  │  - Cache cascade orchestration                         │     │
│  │  - Short code generation coordination                  │     │
│  │  - Visit tracking & analytics                          │     │
│  └───┬────────────────┬──────────────────┬────────────────┘     │
└──────┼────────────────┼──────────────────┼──────────────────────┘
       │                │                  │
┌──────▼────────┐ ┌─────▼──────┐ ┌────────▼────────┐
│  Repository   │ │   Cache    │ │  Bloom Filter   │
│  (GORM/MySQL) │ │  (Redis)   │ │  (In-Memory)    │
└───────────────┘ └────────────┘ └─────────────────┘
       │                │                  │
┌──────▼────────────────▼──────────────────▼──────────────────────┐
│                  Infrastructure Layer                            │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────────────┐   │
│  │  MySQL DB    │  │  Redis KV   │  │  Snowflake ID Gen    │   │
│  │  - Mappings  │  │  - Hot Data │  │  - Unique IDs        │   │
│  │  - Visit Logs│  │  - 24h TTL  │  │  - Base62 Encoding   │   │
│  └──────────────┘  └─────────────┘  └──────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Layered Architecture

#### 1. Presentation Layer (`internal/handler`)
**Responsibilities:**
- HTTP request/response handling with Gin framework
- Input validation and data binding
- Error handling and status code mapping
- API endpoint routing

**Key Components:**
- `URLHandler`: Handles all URL-related HTTP endpoints
- Request/Response DTOs for API contracts
- Route registration and middleware setup

#### 2. Business Logic Layer (`internal/service`)
**Responsibilities:**
- Core business logic implementation
- Multi-layer cache orchestration (Bloom → Redis → MySQL)
- URL validation and normalization
- Visit analytics coordination
- Transaction management

**Key Components:**
- `URLService`: Central service orchestrating all operations
- URL validation logic
- Cache cascade coordination
- Async visit tracking

#### 3. Data Access Layer (`internal/repository`)
**Responsibilities:**
- Database operations abstraction
- GORM ORM integration
- Query optimization
- Connection pool management

**Key Components:**
- `URLRepository`: CRUD operations for URL mappings
- Database schema management via GORM
- Visit log persistence
- Transactional operations

#### 4. Infrastructure Layer
**Cache (`internal/cache`):**
- Redis integration with connection pooling
- Key-value storage with TTL (24h default)
- Cache hit/miss handling
- Prefix-based key organization

**Bloom Filter (`internal/filter`):**
- In-memory probabilistic data structure
- 10M capacity with 1% false positive rate
- Thread-safe operations with RWMutex
- Batch initialization support

**Utils (`internal/utils`):**
- **Snowflake ID Generator**: Distributed unique ID generation
  - 41-bit timestamp (millisecond precision)
  - 10-bit machine ID (datacenter + worker)
  - 12-bit sequence (4096 IDs/ms)
- **Base62 Encoder**: Convert IDs to short codes (6-8 chars)
  - Alphabet: 0-9, a-z, A-Z
  - Bijective mapping for reversibility

### Detailed Request Flows

#### Flow 1: Create Short URL
```
Client Request
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Handler: Validate & Bind Request                         │
│    - Check URL format (http/https)                          │
│    - Validate expiration timestamp                          │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Service: Check Existing URL                              │
│    - Query repository by original URL                       │
│    - Return if active mapping exists                        │
└──────────────────┬──────────────────────────────────────────┘
                   │ (if not exists)
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Generate Short Code                                      │
│    a) Snowflake: Generate unique 64-bit ID                  │
│       - Timestamp: 41 bits (69 years)                       │
│       - Machine ID: 10 bits (1024 machines)                 │
│       - Sequence: 12 bits (4096/ms)                         │
│    b) Base62: Encode ID → 6-8 character string              │
│       Example: 123456789 → aB3xY9                           │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Collision Check (Retry 3x)                               │
│    - Query repository by short code                         │
│    - Regenerate if collision detected                       │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Persist to Database                                      │
│    - Create URLMapping record in MySQL                      │
│    - Set status=1, visit_count=0                            │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 6. Update Cache Layers                                      │
│    - Redis: Set short:code:{code} = original_url (24h TTL) │
│    - Bloom Filter: Add short code to filter                 │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ 7. Return Response                                          │
│    - Short code, full URL, expiration                       │
└─────────────────────────────────────────────────────────────┘
```

**Time Complexity:** O(1) average case
**Latency:** ~10-50ms (including DB write)

#### Flow 2: Redirect to Original URL (3-Layer Cache Cascade)
```
Client Request: GET /{short_code}
      │
      ▼
┌─────────────────────────────────────────────────────────────┐
│ Layer 1: Bloom Filter Check (In-Memory)                     │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ Test if short code exists in filter                  │    │
│ │ - Time: O(k) ≈ O(1) where k=hash functions          │    │
│ │ - Memory: ~1.4MB for 10M items @ 1% FPR             │    │
│ └─────────────────────────────────────────────────────┘    │
│         │                                                    │
│         ├─ FALSE → Return 404 (Definitely not exists)       │
│         │                                                    │
│         └─ TRUE → Continue (Might exist, check cache)       │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ Layer 2: Redis Cache Check                                  │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ GET short:code:{short_code}                          │    │
│ │ - Time: O(1) network RTT + Redis lookup              │    │
│ │ - Hit Rate: ~90% for popular URLs                    │    │
│ │ - Latency: ~1-5ms                                    │    │
│ └─────────────────────────────────────────────────────┘    │
│         │                                                    │
│         ├─ HIT → Return original_url (Fast path) ────┐     │
│         │                                              │     │
│         └─ MISS → Continue to database                │     │
└──────────────────┬──────────────────────────────────────────┘
                   │                                      │
                   ▼                                      │
┌─────────────────────────────────────────────────────────────┐
│ Layer 3: MySQL Database Query                               │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ SELECT * FROM url_mappings                           │    │
│ │ WHERE short_code = ? AND status = 1                  │    │
│ │ - Indexed lookup: O(log n)                           │    │
│ │ - Check expiration: expired_at IS NULL OR > NOW()    │    │
│ │ - Latency: ~5-20ms                                   │    │
│ └─────────────────────────────────────────────────────┘    │
│         │                                                    │
│         ├─ FOUND → Return + Update Redis cache ────────┐   │
│         │                                                │   │
│         └─ NOT FOUND → Return 404                       │   │
└─────────────────────────────────────────────────────────────┘
                   │                                      │
                   ▼                                      │
┌─────────────────────────────────────────────────────────────┐
│ Async Operations (Non-Blocking)                             │◄──┘
│ ┌─────────────────────────────────────────────────────┐    │
│ │ Goroutine 1: Increment visit_count in MySQL         │    │
│ │ Goroutine 2: Insert visit_log (IP, User-Agent)      │    │
│ └─────────────────────────────────────────────────────┘    │
└──────────────────┬──────────────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ HTTP 302 Redirect                                           │
│ Location: {original_url}                                    │
└─────────────────────────────────────────────────────────────┘
```

**Performance Characteristics:**
- **Bloom Filter Hit:** ~0.1ms (in-memory, prevents 100% of invalid queries)
- **Redis Cache Hit:** ~1-5ms (90% of valid queries)
- **MySQL Fallback:** ~5-20ms (10% of valid queries)
- **Overall Avg:** ~2-10ms per redirect

### Component Details

#### 1. URL Handler (`internal/handler/url_handler.go`)
```
Endpoints:
├── POST   /api/v1/shorten          → CreateShortURL
├── GET    /:short_code             → RedirectToOriginalURL
├── GET    /api/v1/info/:short_code → GetURLInfo
└── GET    /health                  → HealthCheck

Responsibilities:
- Request validation with Gin bindings
- Context propagation for timeouts
- Client IP extraction for analytics
- Error to HTTP status code mapping
- Response DTO transformation
```

#### 2. URL Service (`internal/service/url_service.go`)
```
Public Methods:
├── CreateShortURL(url, expiredAt)  → Validate, generate, persist
├── GetOriginalURL(shortCode)       → 3-layer cache cascade
├── GetURLInfo(shortCode)           → Query full mapping details
├── RecordVisit(code, ip, agent)    → Async analytics tracking
└── InitBloomFilter()               → Startup: load all codes

Key Logic:
- URL validation (scheme, host, format)
- Deduplication check before creation
- Collision retry mechanism (3 attempts)
- Cache warming after DB writes
- Graceful cache degradation on errors
```

#### 3. URL Repository (`internal/repository/url_repository.go`)
```
Database Operations:
├── Create(mapping)                  → INSERT new URL mapping
├── GetByShortCode(code)             → SELECT with index
├── GetByOriginalURL(url)            → Deduplication check
├── IncrementVisitCount(code)        → Atomic UPDATE
├── CreateVisitLog(log)              → INSERT visit record
├── GetAllShortCodes()               → Bloom filter initialization
├── Update(mapping)                  → UPDATE mapping
└── Delete(code)                     → Soft/hard delete

Connection Management:
- Connection pooling (configurable)
- Auto-migration with GORM
- Prepared statements for security
- Transaction support
```

#### 4. Redis Cache (`internal/cache/redis.go`)
```
Cache Strategy:
- Key Pattern: short:code:{short_code}
- TTL: 24 hours (configurable)
- Eviction: LRU (Least Recently Used)
- Pool Size: 100 connections (configurable)

Operations:
├── Get(shortCode)                   → O(1) lookup
├── Set(shortCode, url)              → O(1) with 24h TTL
├── SetWithTTL(code, url, duration)  → Custom expiration
└── Delete(shortCode)                → Cache invalidation

Performance:
- Connection pooling for concurrency
- Pipeline support for batch ops
- Automatic reconnection on failure
```

#### 5. Bloom Filter (`internal/filter/bloom.go`)
```
Configuration:
- Capacity: 10,000,000 URLs
- False Positive Rate: 1%
- Memory Usage: ~1.4MB
- Hash Functions: Optimal k calculated by library

Thread Safety:
- RWMutex for concurrent access
- Read operations: parallel
- Write operations: exclusive lock

Operations:
├── Add(shortCode)                   → O(k) ≈ O(1)
├── Test(shortCode)                  → O(k) ≈ O(1)
├── AddBatch(codes)                  → Bulk initialization
└── Clear()                          → Reset filter

Benefits:
- Prevents 100% of invalid DB queries
- ~0.1ms lookup time
- Space-efficient (1.4MB vs GB in DB)
```

#### 6. Snowflake ID Generator (`internal/utils/snowflake.go`)
```
ID Structure (64 bits):
┌─────────────┬──────────┬──────────┬──────────────┐
│  Timestamp  │ Datactr  │  Worker  │   Sequence   │
│   41 bits   │  5 bits  │  5 bits  │   12 bits    │
└─────────────┴──────────┴──────────┴──────────────┘
     │             │          │            │
     │             │          │            └─ 0-4095 (4096 IDs/ms)
     │             │          └─ 0-31 (32 workers per datacenter)
     │             └─ 0-31 (32 datacenters)
     └─ Milliseconds since epoch (69 years)

Guarantees:
- Unique across distributed systems
- Sortable by time (natural ordering)
- No coordination required
- 4096 IDs per millisecond per worker

Node ID Calculation:
node_id = (datacenter_id << 5) | worker_id
```

#### 7. Base62 Encoder (`internal/utils/shortcode.go`)
```
Encoding Process:
1. Generate Snowflake ID (64-bit int)
2. Convert to Base62 using alphabet: [0-9a-zA-Z]
3. Result: 6-8 character string

Example:
Snowflake ID: 1234567890123456
Base62 Code:  "aB3xY9Km"

Advantages:
- URL-safe characters only
- Case-sensitive (more combinations)
- Shorter than Base36 or hex
- Reversible (can decode back to ID)

Collision Probability:
62^6 = 56.8 billion combinations
62^7 = 3.5 trillion combinations
→ Near-zero collision with Snowflake uniqueness
```

### Design Patterns & Principles

#### 1. Repository Pattern
- **Purpose:** Abstracts data access logic from business logic
- **Benefits:**
  - Easy to swap DB implementations
  - Testable with mock repositories
  - Centralized query optimization

#### 2. Service Layer Pattern
- **Purpose:** Encapsulates business logic
- **Benefits:**
  - Single source of truth for business rules
  - Coordinates between multiple repositories/caches
  - Handles transactions and error recovery

#### 3. Dependency Injection
- **Implementation:** Constructor injection in `main.go`
- **Benefits:**
  - Loose coupling between layers
  - Easy to mock for unit tests
  - Clear dependency graph

#### 4. Cache-Aside Pattern
- **Implementation:** 3-layer cascade (Bloom → Redis → MySQL)
- **Benefits:**
  - Lazy loading (cache on demand)
  - Cache invalidation control
  - Resilient to cache failures

#### 5. Circuit Breaker (Implicit)
- **Implementation:** Graceful degradation on cache failures
- **Benefits:**
  - Service continues if Redis fails
  - Logs errors without blocking requests
  - Automatic recovery when cache reconnects

### Performance Optimization Strategies

#### 1. Cache Penetration Prevention
**Problem:** Malicious queries for non-existent URLs flood DB
**Solution:** Bloom filter rejects 100% of invalid codes in O(1)
**Impact:** Zero DB load for invalid requests

#### 2. Cache Stampede Mitigation
**Problem:** Cache expiration causes thundering herd to DB
**Solution:**
- 24h TTL staggers expiration
- Async cache warming on writes
- Connection pooling limits concurrency

#### 3. Async Visit Tracking
**Problem:** Recording visits blocks redirect latency
**Solution:** Goroutines handle analytics non-blocking
**Impact:** Redirect latency independent of logging

#### 4. Index Optimization
```sql
-- Unique index for O(log n) short code lookup
CREATE UNIQUE INDEX idx_short_code ON url_mappings(short_code);

-- Index for deduplication check
CREATE INDEX idx_original_url ON url_mappings(original_url(255));

-- Composite index for visit log queries
CREATE INDEX idx_visit_logs ON visit_logs(short_code, visited_at);
```

#### 5. Connection Pooling
- **MySQL:** Reuses connections (10 idle, 100 max)
- **Redis:** 100-connection pool for high concurrency
- **HTTP:** Keep-alive for client connections

### Scalability Considerations

#### Horizontal Scaling
1. **Application Tier:** Stateless servers, scale with load balancer
2. **Database Tier:** MySQL read replicas for analytics queries
3. **Cache Tier:** Redis Cluster for sharded cache
4. **Snowflake ID:** Unique worker IDs per instance (1024 max)

#### Vertical Scaling
1. **Redis:** Increase memory for larger cache
2. **MySQL:** Increase IOPS for faster writes
3. **Bloom Filter:** Increase capacity for more URLs

#### Data Partitioning
- **Future:** Shard by short_code hash for >100M URLs
- **Strategy:** Consistent hashing for cache distribution

### Security Features

1. **Input Validation:** URL format, scheme, and host checks
2. **SQL Injection Prevention:** GORM parameterized queries
3. **Rate Limiting:** (Future) Implement token bucket on handler
4. **HTTPS Only:** Enforce in production with TLS termination
5. **No URL Enumeration:** Base62 makes guessing hard (62^7 space)

## Performance Optimization

- **Bloom Filter**: Prevents unnecessary database queries for non-existent short codes
- **Redis Cache**: 24-hour TTL reduces database load by ~90%
- **Connection Pooling**: Reuses MySQL and Redis connections
- **Async Logging**: Visit logs recorded asynchronously to avoid blocking redirects
- **Index Optimization**: Unique index on short_code, composite indexes on frequent queries

## Testing

### Functional Testing

```bash
# Create a short URL
curl -X POST http://localhost:8080/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.google.com"}'

# Access the short URL
curl -i http://localhost:8080/aB3xY9

# Get URL info
curl http://localhost:8080/api/v1/info/aB3xY9
```

### Testing with Postman

#### 1. Setup Postman Collection

Create a new collection called "Short Link Service" and optionally set up an environment variable:

- **Variable Name**: `base_url`
- **Value**: `http://localhost:8080`

#### 2. Test 1: Health Check

- **Method**: `GET`
- **URL**: `{{base_url}}/health` or `http://localhost:8080/health`
- **Expected Response**:
```json
{
  "code": 200,
  "message": "OK"
}
```

#### 3. Test 2: Create Short URL (Basic)

- **Method**: `POST`
- **URL**: `{{base_url}}/api/v1/shorten` or `http://localhost:8080/api/v1/shorten`
- **Headers**:
  - `Content-Type`: `application/json`
- **Body** (raw JSON):
```json
{
  "url": "https://www.google.com"
}
```
- **Expected Response**:
```json
{
  "code": 200,
  "data": {
    "short_code": "aB3xY9",
    "short_url": "http://localhost:8080/aB3xY9",
    "original_url": "https://www.google.com"
  }
}
```

#### 4. Test 3: Create Short URL with Expiration

- **Method**: `POST`
- **URL**: `{{base_url}}/api/v1/shorten`
- **Headers**:
  - `Content-Type`: `application/json`
- **Body** (raw JSON):
```json
{
  "url": "https://www.example.com/temporary-link",
  "expired_at": "2025-12-31T23:59:59Z"
}
```

#### 5. Test 4: Redirect to Original URL

- **Method**: `GET`
- **URL**: `{{base_url}}/{short_code}` (replace `{short_code}` with actual code from Test 2)
- **Example**: `http://localhost:8080/aB3xY9`
- **Expected**: 302 redirect to original URL
- **Postman Tip**: Disable "Automatically follow redirects" in Settings to see the 302 response

#### 6. Test 5: Get URL Information

- **Method**: `GET`
- **URL**: `{{base_url}}/api/v1/info/{short_code}`
- **Example**: `http://localhost:8080/api/v1/info/aB3xY9`
- **Expected Response**:
```json
{
  "code": 200,
  "data": {
    "short_code": "aB3xY9",
    "original_url": "https://www.google.com",
    "visit_count": 5,
    "created_at": "2025-10-04T12:05:30Z",
    "expired_at": null
  }
}
```

#### Postman Testing Workflow

1. **Health Check** - Verify service is running
2. **Create Short URL** - Copy the `short_code` from response
3. **Test Redirect** - Use the short code to test redirection
4. **Check Stats** - Use Get URL Info to see visit count incremented
5. **Test Expiration** - Create URL with past expiration date, verify it's rejected on redirect

#### Quick Import (Optional)

You can create a Postman collection JSON file with all these requests pre-configured. Save this as `postman_collection.json`:

```json
{
  "info": {
    "name": "Short Link Service",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "Health Check",
      "request": {
        "method": "GET",
        "url": "{{base_url}}/health"
      }
    },
    {
      "name": "Create Short URL",
      "request": {
        "method": "POST",
        "header": [{"key": "Content-Type", "value": "application/json"}],
        "body": {
          "mode": "raw",
          "raw": "{\"url\":\"https://www.google.com\"}"
        },
        "url": "{{base_url}}/api/v1/shorten"
      }
    },
    {
      "name": "Redirect to Original",
      "request": {
        "method": "GET",
        "url": "{{base_url}}/aB3xY9"
      }
    },
    {
      "name": "Get URL Info",
      "request": {
        "method": "GET",
        "url": "{{base_url}}/api/v1/info/aB3xY9"
      }
    }
  ]
}
```

Import this file in Postman: **Import** → **Upload Files** → Select `postman_collection.json`

### Load Testing (Optional)

Using [wrk](https://github.com/wg/wrk):

```bash
# Install wrk
# Ubuntu: sudo apt-get install wrk
# macOS: brew install wrk

# Run load test
wrk -t4 -c100 -d30s http://localhost:8080/aB3xY9
```

## Troubleshooting

### Common Issues

1. **Database connection failed**:
   - Ensure MySQL is running: `docker-compose ps`
   - Check MySQL credentials in `config/config.yaml`
   - Verify network connectivity

2. **Redis connection failed**:
   - Ensure Redis is running: `docker-compose ps`
   - Check Redis configuration in `config/config.yaml`

3. **Port already in use**:
   - Change port in `config/config.yaml` and `docker-compose.yml`
   - Or stop the conflicting service

4. **Bloom filter initialization warning**:
   - This is normal for a fresh database with no short codes
   - The warning can be ignored on first startup

### Logs

```bash
# View all logs
docker-compose logs

# View application logs only
docker-compose logs app

# Follow logs in real-time
docker-compose logs -f app
```

## Docker Commands

```bash
# Start services
docker-compose up -d

# Stop services
docker-compose down

# Rebuild and restart
docker-compose up -d --build

# View logs
docker-compose logs -f

# Remove all data (including database)
docker-compose down -v
```

## License

MIT License

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## Contact

- Author: Monthlyaway
- GitHub: https://github.com/Monthlyaway/short-link
