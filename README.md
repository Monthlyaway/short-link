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

### Request Flow

1. **Create Short URL**:
   - Validate URL format
   - Check for existing URL in database
   - Generate Snowflake ID
   - Encode to Base62 short code
   - Save to MySQL
   - Cache in Redis (24h TTL)
   - Add to Bloom filter

2. **Redirect Flow**:
   - Check Bloom filter (O(1), prevents cache penetration)
   - Check Redis cache (cache hit ~90%)
   - Query MySQL (cache miss)
   - Update Redis cache
   - Record visit asynchronously
   - Redirect to original URL

### Key Design Patterns

- **Repository Pattern**: Separation of data access logic
- **Service Layer**: Business logic isolation
- **Dependency Injection**: Loose coupling between components
- **Graceful Shutdown**: Proper resource cleanup on termination

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
