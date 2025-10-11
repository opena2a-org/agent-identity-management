# AIM Performance Guide

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [Performance Targets](#performance-targets)
3. [Backend Optimization](#backend-optimization)
4. [Database Optimization](#database-optimization)
5. [Caching Strategy](#caching-strategy)
6. [Frontend Optimization](#frontend-optimization)
7. [Load Testing](#load-testing)
8. [Monitoring & Profiling](#monitoring--profiling)
9. [Scalability](#scalability)

---

## Overview

AIM is designed for **high performance** and **horizontal scalability**. This document provides comprehensive guidance on:

- Meeting performance targets (< 100ms API response)
- Optimizing database queries and indexing
- Implementing effective caching strategies
- Load testing and capacity planning
- Monitoring performance metrics
- Scaling to handle 1000+ concurrent users

### Performance Philosophy

**"Performance is a Feature"** - Fast response times improve user experience, reduce costs, and enable real-time security monitoring.

**Key Principles**:
1. **Measure First**: Profile before optimizing
2. **Cache Aggressively**: Redis for hot data
3. **Index Strategically**: Database indexes for common queries
4. **Scale Horizontally**: Add more instances, not bigger instances
5. **Monitor Continuously**: Real-time performance dashboards

---

## Performance Targets

### API Response Times (p95)

| Endpoint | Target (p95) | Critical Path |
|----------|-------------|---------------|
| `GET /api/v1/agents` | < 50ms | Database query + JSON serialization |
| `POST /api/v1/agents` | < 100ms | Key generation + database insert + audit log |
| `POST /api/v1/agents/{id}/verify` | < 200ms | Crypto verification + trust score calc + audit log |
| `GET /api/v1/audit-logs` | < 100ms | Database query + filtering + pagination |
| `GET /api/v1/mcp-servers` | < 50ms | Database query + JSON serialization |
| `POST /api/v1/mcp-servers` | < 100ms | Database insert + capability detection + audit log |
| `GET /api/v1/dashboard/stats` | < 50ms | Cached aggregations + Redis lookup |

**p95 = 95th percentile** - 95% of requests must complete within target time.

### Throughput Targets

| Configuration | Requests/Second | Concurrent Users |
|---------------|----------------|------------------|
| Single Backend Instance | 1,000 RPS | 100 users |
| 3 Backend Instances | 3,000 RPS | 300 users |
| 10 Backend Instances | 10,000 RPS | 1,000 users |

### Resource Utilization Targets

| Resource | Target | Scaling Trigger |
|----------|--------|-----------------|
| CPU Usage | < 70% | > 80% for 5 minutes |
| Memory Usage | < 80% | > 90% for 5 minutes |
| Database Connections | < 80% of pool | > 90% |
| Redis Memory | < 70% | > 85% |

---

## Backend Optimization

### Go Performance Best Practices

**1. Minimize Allocations**

```go
// BAD - creates new slice on every call
func getAgentIDs(agents []Agent) []string {
    ids := []string{}
    for _, agent := range agents {
        ids = append(ids, agent.ID.String())
    }
    return ids
}

// GOOD - pre-allocate slice
func getAgentIDs(agents []Agent) []string {
    ids := make([]string, len(agents))
    for i, agent := range agents {
        ids[i] = agent.ID.String()
    }
    return ids
}
```

**2. Use Pointer Receivers for Large Structs**

```go
// BAD - copies entire struct on every call
func (a Agent) UpdateTrustScore(score float64) {
    a.TrustScore = score
}

// GOOD - uses pointer, no copy
func (a *Agent) UpdateTrustScore(score float64) {
    a.TrustScore = score
}
```

**3. Avoid String Concatenation in Loops**

```go
// BAD - creates new string on every iteration
var result string
for _, item := range items {
    result += item + ", "
}

// GOOD - uses strings.Builder (efficient)
var builder strings.Builder
for _, item := range items {
    builder.WriteString(item)
    builder.WriteString(", ")
}
result := builder.String()
```

**4. Use sync.Pool for Frequent Allocations**

```go
// Pool for JSON encoders
var encoderPool = sync.Pool{
    New: func() interface{} {
        return json.NewEncoder(nil)
    },
}

// Get encoder from pool
encoder := encoderPool.Get().(*json.Encoder)
defer encoderPool.Put(encoder)

// Use encoder
encoder.Encode(data)
```

### Concurrency Optimization

**1. Use Worker Pools for Batch Operations**

```go
// Process 1000 agents concurrently with 10 workers
func processBatch(agents []Agent) {
    jobs := make(chan Agent, len(agents))
    results := make(chan Result, len(agents))

    // Start 10 workers
    for w := 0; w < 10; w++ {
        go worker(jobs, results)
    }

    // Send jobs
    for _, agent := range agents {
        jobs <- agent
    }
    close(jobs)

    // Collect results
    for i := 0; i < len(agents); i++ {
        <-results
    }
}
```

**2. Use Context for Timeout and Cancellation**

```go
func verifyAgentWithTimeout(agentID uuid.UUID) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    return verifyAgent(ctx, agentID)
}
```

---

## Database Optimization

### Indexing Strategy

**Critical Indexes** (already implemented):

```sql
-- Agents table
CREATE INDEX idx_agents_organization_id ON agents(organization_id);
CREATE INDEX idx_agents_created_at ON agents(created_at);
CREATE INDEX idx_agents_trust_score ON agents(trust_score);

-- MCP Servers table
CREATE INDEX idx_mcp_servers_organization_id ON mcp_servers(organization_id);
CREATE INDEX idx_mcp_servers_trust_score ON mcp_servers(trust_score);

-- Audit Logs table
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_organization_id ON audit_logs(organization_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);

-- Composite indexes for common queries
CREATE INDEX idx_audit_logs_org_time ON audit_logs(organization_id, timestamp DESC);
CREATE INDEX idx_agents_org_trust ON agents(organization_id, trust_score DESC);
```

**Index Maintenance**:

```sql
-- Analyze tables weekly (updates statistics)
ANALYZE agents;
ANALYZE mcp_servers;
ANALYZE audit_logs;

-- Vacuum tables monthly (reclaims space)
VACUUM ANALYZE agents;
VACUUM ANALYZE mcp_servers;
VACUUM ANALYZE audit_logs;
```

### Query Optimization

**1. Use LIMIT and OFFSET for Pagination**

```go
// Get agents with pagination
func (r *PostgresAgentRepository) List(ctx context.Context, orgID uuid.UUID, page, pageSize int) ([]Agent, error) {
    offset := (page - 1) * pageSize

    query := `
        SELECT id, name, agent_type, trust_score, created_at
        FROM agents
        WHERE organization_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `

    rows, err := r.db.QueryContext(ctx, query, orgID, pageSize, offset)
    // ...
}
```

**2. Use SELECT Specific Columns (not SELECT *)**

```go
// BAD - fetches all columns (inefficient)
query := "SELECT * FROM agents WHERE organization_id = $1"

// GOOD - fetches only needed columns
query := "SELECT id, name, trust_score FROM agents WHERE organization_id = $1"
```

**3. Use EXISTS for Conditional Checks**

```sql
-- BAD - fetches entire row just to check existence
SELECT * FROM agents WHERE id = $1;

-- GOOD - only checks existence (faster)
SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1);
```

**4. Use EXPLAIN ANALYZE to Profile Queries**

```sql
-- Analyze query performance
EXPLAIN ANALYZE
SELECT id, name, trust_score
FROM agents
WHERE organization_id = 'uuid'
  AND trust_score >= 75
ORDER BY trust_score DESC
LIMIT 10;

-- Output shows:
-- - Execution time
-- - Index usage
-- - Rows scanned
-- - Sort/filter operations
```

### Connection Pooling

**PostgreSQL Connection Pool Configuration**:

```go
// Configure connection pool
db.SetMaxOpenConns(25)        // Max concurrent connections
db.SetMaxIdleConns(10)        // Idle connections in pool
db.SetConnMaxLifetime(time.Hour) // Max connection lifetime

// Monitor pool stats
stats := db.Stats()
log.Printf("Open connections: %d", stats.OpenConnections)
log.Printf("In use: %d", stats.InUse)
log.Printf("Idle: %d", stats.Idle)
```

**Sizing Guidelines**:
- `MaxOpenConns`: 25 per backend instance (prevents connection exhaustion)
- `MaxIdleConns`: 10 (reuse connections efficiently)
- `ConnMaxLifetime`: 1 hour (prevents stale connections)

### Batch Operations

**Bulk Inserts with COPY**:

```go
// Insert 10,000 audit logs efficiently
func bulkInsertAuditLogs(logs []AuditLog) error {
    // Use COPY for bulk insert (100x faster than individual INSERTs)
    stmt, err := db.Prepare(pq.CopyIn("audit_logs", "id", "timestamp", "user_id", "action", "resource_type"))
    if err != nil {
        return err
    }

    for _, log := range logs {
        _, err = stmt.Exec(log.ID, log.Timestamp, log.UserID, log.Action, log.ResourceType)
        if err != nil {
            return err
        }
    }

    _, err = stmt.Exec()
    if err != nil {
        return err
    }

    return stmt.Close()
}
```

---

## Caching Strategy

### Redis Caching Architecture

**Cache Hierarchy**:
```
Request → In-Memory Cache → Redis Cache → Database
            (L1)              (L2)         (L3)
```

### What to Cache

| Data Type | TTL | Invalidation |
|-----------|-----|-------------|
| Dashboard Stats | 5 minutes | Manual refresh |
| User Sessions | 24 hours | Logout |
| Agent Metadata | 15 minutes | Agent update |
| MCP Server List | 10 minutes | Server update |
| Trust Scores | 1 minute | Trust recalculation |
| API Rate Limits | 1 minute | Rolling window |

### Caching Implementation

**1. Dashboard Stats Caching**

```go
func (s *DashboardService) GetStats(ctx context.Context, orgID uuid.UUID) (*Stats, error) {
    // Try Redis cache first
    cacheKey := fmt.Sprintf("dashboard:stats:%s", orgID)
    cached, err := s.redis.Get(ctx, cacheKey).Result()

    if err == nil {
        // Cache hit
        var stats Stats
        json.Unmarshal([]byte(cached), &stats)
        return &stats, nil
    }

    // Cache miss - fetch from database
    stats, err := s.calculateStats(ctx, orgID)
    if err != nil {
        return nil, err
    }

    // Store in cache for 5 minutes
    data, _ := json.Marshal(stats)
    s.redis.Set(ctx, cacheKey, data, 5*time.Minute)

    return stats, nil
}
```

**2. Session Caching**

```go
// Store session in Redis
func storeSession(sessionID string, userID uuid.UUID, expiration time.Duration) error {
    key := fmt.Sprintf("session:%s", sessionID)
    return redis.Set(ctx, key, userID.String(), expiration).Err()
}

// Retrieve session from Redis
func getSession(sessionID string) (uuid.UUID, error) {
    key := fmt.Sprintf("session:%s", sessionID)
    val, err := redis.Get(ctx, key).Result()
    if err != nil {
        return uuid.Nil, err
    }
    return uuid.Parse(val)
}
```

**3. Rate Limiting with Redis**

```go
// Redis-based rate limiting (100 requests per minute)
func rateLimitCheck(userID uuid.UUID) (bool, error) {
    key := fmt.Sprintf("rate_limit:%s:%d", userID, time.Now().Unix()/60)

    count, err := redis.Incr(ctx, key).Result()
    if err != nil {
        return false, err
    }

    if count == 1 {
        redis.Expire(ctx, key, time.Minute)
    }

    return count <= 100, nil
}
```

### Cache Invalidation Strategies

**1. Time-Based (TTL)**

```go
// Cache expires automatically after TTL
redis.Set(ctx, key, value, 5*time.Minute)
```

**2. Event-Based (Manual Invalidation)**

```go
// Invalidate cache when agent is updated
func (s *AgentService) UpdateAgent(ctx context.Context, agent *Agent) error {
    err := s.repo.Update(ctx, agent)
    if err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("agent:%s", agent.ID)
    s.redis.Del(ctx, cacheKey)

    return nil
}
```

**3. Write-Through Cache**

```go
// Update both cache and database
func (s *AgentService) UpdateTrustScore(ctx context.Context, agentID uuid.UUID, score float64) error {
    // Update database
    err := s.repo.UpdateTrustScore(ctx, agentID, score)
    if err != nil {
        return err
    }

    // Update cache immediately
    cacheKey := fmt.Sprintf("trust_score:%s", agentID)
    s.redis.Set(ctx, cacheKey, score, time.Minute)

    return nil
}
```

---

## Frontend Optimization

### Next.js Performance

**1. Server Components for Data Fetching**

```typescript
// Server Component - no client-side JavaScript
export default async function DashboardPage() {
  // Data fetched on server
  const stats = await fetchStats();

  return (
    <div>
      <h1>Dashboard</h1>
      <StatsDisplay stats={stats} />
    </div>
  );
}
```

**2. Dynamic Imports for Code Splitting**

```typescript
// Load heavy component only when needed
import dynamic from 'next/dynamic';

const HeavyChart = dynamic(() => import('@/components/charts/HeavyChart'), {
  loading: () => <Spinner />,
  ssr: false, // Don't render on server
});
```

**3. Image Optimization**

```typescript
// Use Next.js Image component (automatic optimization)
import Image from 'next/image';

<Image
  src="/logo.png"
  alt="Logo"
  width={200}
  height={100}
  priority // Load immediately for LCP
/>
```

**4. API Route Caching**

```typescript
// Cache API responses
export async function GET(request: Request) {
  const stats = await getStats();

  return Response.json(stats, {
    headers: {
      'Cache-Control': 'public, s-maxage=300, stale-while-revalidate=600',
    },
  });
}
```

### React Performance

**1. Memoization**

```typescript
// Prevent unnecessary re-renders
const MemoizedComponent = React.memo(AgentList);

// Memoize expensive calculations
const sortedAgents = useMemo(() => {
  return agents.sort((a, b) => b.trustScore - a.trustScore);
}, [agents]);

// Memoize callbacks
const handleClick = useCallback(() => {
  console.log('Clicked');
}, []);
```

**2. Virtual Scrolling for Large Lists**

```typescript
import { useVirtualizer } from '@tanstack/react-virtual';

// Render only visible items (not all 10,000)
function AgentList({ agents }) {
  const virtualizer = useVirtualizer({
    count: agents.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 50, // Row height
  });

  return (
    <div ref={parentRef}>
      {virtualizer.getVirtualItems().map((virtualRow) => (
        <div key={virtualRow.index}>
          {agents[virtualRow.index].name}
        </div>
      ))}
    </div>
  );
}
```

---

## Load Testing

### K6 Load Testing

**Installation**:
```bash
# Install k6
brew install k6  # macOS
# or
curl https://github.com/grafana/k6/releases/download/v0.47.0/k6-v0.47.0-linux-amd64.tar.gz -L | tar xvz
```

**Load Test Script** (`tests/load/api_load_test.js`):

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 200 },  // Ramp up to 200 users
    { duration: '5m', target: 200 },  // Stay at 200 users
    { duration: '2m', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    'http_req_duration': ['p(95)<100'], // 95% of requests < 100ms
    'http_req_failed': ['rate<0.01'],   // Error rate < 1%
  },
};

const BASE_URL = 'http://localhost:8080';
const TOKEN = 'your-jwt-token-here';

export default function() {
  // Test GET /api/v1/agents
  const agentsRes = http.get(`${BASE_URL}/api/v1/agents`, {
    headers: { 'Authorization': `Bearer ${TOKEN}` },
  });

  check(agentsRes, {
    'status is 200': (r) => r.status === 200,
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  sleep(1); // Think time between requests
}
```

**Run Load Test**:
```bash
# Run load test
k6 run tests/load/api_load_test.js

# Run with custom parameters
k6 run --vus 500 --duration 30s tests/load/api_load_test.js

# Run and save results
k6 run --out json=results.json tests/load/api_load_test.js
```

**Expected Results**:
```
     ✓ status is 200
     ✓ response time < 100ms

     checks.........................: 100.00% ✓ 12000  ✗ 0
     data_received..................: 24 MB   800 kB/s
     data_sent......................: 2.4 MB  80 kB/s
     http_req_duration..............: avg=45ms min=12ms med=38ms max=250ms p(95)=85ms p(99)=120ms
     http_req_failed................: 0.00%   ✓ 0      ✗ 12000
     http_reqs......................: 12000   400/s
     iteration_duration.............: avg=1.04s min=1.01s med=1.03s max=1.25s
     iterations.....................: 12000   400/s
     vus............................: 100     min=0    max=200
     vus_max........................: 200     min=200  max=200
```

---

## Monitoring & Profiling

### Prometheus Metrics

**Key Metrics to Monitor**:

```go
var (
    // HTTP request metrics
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aim_http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "aim_http_request_duration_seconds",
            Help: "HTTP request duration in seconds",
            Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 5},
        },
        []string{"method", "path"},
    )

    // Database metrics
    dbQueriesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aim_db_queries_total",
            Help: "Total number of database queries",
        },
        []string{"table", "operation"},
    )

    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "aim_db_query_duration_seconds",
            Help: "Database query duration in seconds",
            Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1},
        },
        []string{"table", "operation"},
    )

    // Trust score metrics
    trustScoreChanges = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "aim_trust_score_changes",
            Help: "Trust score changes over time",
            Buckets: []float64{-20, -10, -5, 0, 5, 10, 20},
        },
        []string{"agent_id"},
    )
)
```

### Grafana Dashboards

**Dashboard Panels**:
1. **API Response Times** (p50, p95, p99)
2. **Request Rate** (requests per second)
3. **Error Rate** (% of failed requests)
4. **Database Query Times** (p50, p95, p99)
5. **Cache Hit Rate** (% of cache hits)
6. **CPU Usage** (% per instance)
7. **Memory Usage** (MB per instance)
8. **Trust Score Trends** (average score over time)

### Go Profiling

**CPU Profiling**:
```bash
# Start server with profiling enabled
go run cmd/server/main.go -cpuprofile=cpu.prof

# Run load test for 60 seconds
k6 run --duration 60s tests/load/api_load_test.js

# Stop server (Ctrl+C)
# Analyze profile
go tool pprof cpu.prof

# Interactive commands:
# top - Show top functions by CPU time
# list <function> - Show source code
# web - Open in browser (requires graphviz)
```

**Memory Profiling**:
```bash
# Capture heap profile
curl http://localhost:8080/debug/pprof/heap > heap.prof

# Analyze profile
go tool pprof heap.prof

# Interactive commands:
# top - Show top functions by memory allocation
# list <function> - Show source code
```

---

## Scalability

### Horizontal Scaling

**Kubernetes Auto-Scaling**:

```yaml
# HorizontalPodAutoscaler for backend
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: aim-backend-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: aim-backend
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

**Scaling Triggers**:
- CPU > 70% for 2 minutes → Scale up
- CPU < 30% for 5 minutes → Scale down
- Memory > 80% → Scale up immediately

### Database Scaling

**Read Replicas**:

```go
// Use read replica for queries
func (r *AgentRepository) List(ctx context.Context, orgID uuid.UUID) ([]Agent, error) {
    // Read from replica
    rows, err := r.replicaDB.QueryContext(ctx, query, orgID)
    // ...
}

// Use primary for writes
func (r *AgentRepository) Create(ctx context.Context, agent *Agent) error {
    // Write to primary
    _, err := r.primaryDB.ExecContext(ctx, query, agent.ID, agent.Name, agent.TrustScore)
    // ...
}
```

**Connection Pool Sizing**:
- Primary: 25 connections per backend instance
- Replica: 50 connections per backend instance (read-heavy)

### Redis Scaling

**Redis Cluster Configuration**:

```bash
# 3-node Redis cluster with replication
redis-cli --cluster create \
  redis-1:6379 \
  redis-2:6379 \
  redis-3:6379 \
  --cluster-replicas 1
```

**Client-Side Sharding**:

```go
// Hash key to determine Redis node
func getRedisNode(key string) *redis.Client {
    hash := crc32.ChecksumIEEE([]byte(key))
    nodeIndex := hash % uint32(len(redisNodes))
    return redisNodes[nodeIndex]
}
```

---

## References

- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [Go Performance Tips](https://go.dev/doc/effective_go#performance)
- [PostgreSQL Performance Tuning](https://wiki.postgresql.org/wiki/Performance_Optimization)
- [Redis Best Practices](https://redis.io/docs/management/optimization/)
- [K6 Documentation](https://k6.io/docs/)

---

**Maintained by**: OpenA2A Performance Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026

For performance issues, contact: performance@yourdomain.com
