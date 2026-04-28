#!/usr/bin/env bash
# Smoke test harness for the AIM OTel demo stack.
#
# What this script does, in order:
#   1. Pre-flight: fail fast on port conflicts, missing docker, missing curl.
#   2. Boot the docker-compose stack and wait for each service to be healthy.
#   3. Send a synthetic OTLP trace + metric + log via grpcurl OR via a
#      throwaway Go program against the collector's gRPC endpoint.
#   4. Verify the signal landed:
#        Tempo: GET /api/traces/<trace_id> returns 200 with non-empty body
#        Prometheus: GET /api/v1/query?query=... returns the metric
#        Loki: GET /loki/api/v1/query_range?query=... returns the log line
#   5. Print a one-line PASS / FAIL.
#
# Run: ./smoke-test.sh
# To use custom ports: copy .env.example to .env first, then run.
#
# Exit codes:
#   0 = all signals verified end-to-end
#   1 = port conflict (no boot attempted)
#   2 = docker compose up failed
#   3 = a service failed health check
#   4 = a signal failed to land in its backend
set -euo pipefail

cd "$(dirname "$0")"

# Load .env if present (port overrides).
if [ -f .env ]; then
    # shellcheck disable=SC1091
    set -a; . ./.env; set +a
fi

OTEL_GRPC_PORT="${OTEL_GRPC_PORT:-4317}"
OTEL_HTTP_PORT="${OTEL_HTTP_PORT:-4318}"
TEMPO_HTTP_PORT="${TEMPO_HTTP_PORT:-3200}"
PROMETHEUS_PORT="${PROMETHEUS_PORT:-9090}"
LOKI_PORT="${LOKI_PORT:-3100}"
GRAFANA_PORT="${GRAFANA_PORT:-3001}"

echo "==> AIM OTel demo smoke test"
echo "    OTLP gRPC   : localhost:${OTEL_GRPC_PORT}"
echo "    Tempo HTTP  : localhost:${TEMPO_HTTP_PORT}"
echo "    Prometheus  : localhost:${PROMETHEUS_PORT}"
echo "    Loki        : localhost:${LOKI_PORT}"
echo "    Grafana     : localhost:${GRAFANA_PORT}"

# 1. Pre-flight (port check happens AFTER cleanup so our own prior containers
#    do not register as conflicts).
echo
echo "==> [1/5] Pre-flight checks + cleanup of prior runs"
command -v docker >/dev/null || { echo "FAIL: docker not on PATH"; exit 1; }
command -v curl >/dev/null   || { echo "FAIL: curl not on PATH"; exit 1; }

# Reap any prior aim-otel-demo containers (idempotent — even if compose can't
# find them because the network was deleted, the named container survives).
docker compose down --remove-orphans >/dev/null 2>&1 || true
for c in aim-otel-collector aim-tempo aim-loki aim-prometheus aim-grafana; do
    docker rm -f "$c" >/dev/null 2>&1 || true
done

conflicts=0
for p in "$OTEL_GRPC_PORT" "$OTEL_HTTP_PORT" "$TEMPO_HTTP_PORT" "$PROMETHEUS_PORT" "$LOKI_PORT" "$GRAFANA_PORT"; do
    if lsof -i ":$p" -sTCP:LISTEN >/dev/null 2>&1; then
        echo "    BUSY: port $p is already in use (by something other than this stack)"
        conflicts=$((conflicts+1))
    fi
done
if [ "$conflicts" -gt 0 ]; then
    echo "FAIL: $conflicts port conflict(s). Copy .env.example to .env and pick free ports."
    exit 1
fi
echo "    ports clear, prior runs cleaned up"

# 2. Boot stack
echo
echo "==> [2/5] docker compose up -d"
docker compose up -d 2>&1 | tail -10 || { echo "FAIL: docker compose up failed"; exit 2; }

# 3. Wait for services to be healthy.
wait_http() {
    local name="$1" url="$2" max_attempts=60 attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -sf "$url" >/dev/null 2>&1; then
            echo "    ${name} ready"
            return 0
        fi
        attempt=$((attempt+1))
        sleep 2
    done
    echo "FAIL: ${name} did not become ready at ${url} after ${max_attempts}*2s"
    return 3
}
echo
echo "==> [3/5] Wait for services"
wait_http tempo      "http://localhost:${TEMPO_HTTP_PORT}/ready"        || exit 3
wait_http prometheus "http://localhost:${PROMETHEUS_PORT}/-/ready"      || exit 3
wait_http loki       "http://localhost:${LOKI_PORT}/ready"              || exit 3
wait_http grafana    "http://localhost:${GRAFANA_PORT}/api/health"      || exit 3
# Collector exposes a health_check extension on 13133.
wait_http collector  "http://localhost:${OTEL_HEALTH_PORT:-13133}/" || exit 3

# 4. Send a synthetic signal via the otelgen Go helper (built ad-hoc).
echo
echo "==> [4/5] Send synthetic trace + metric + log"
TRACE_ID=$(./gen-signal.sh "$OTEL_GRPC_PORT") || { echo "FAIL: gen-signal.sh failed"; exit 4; }
echo "    trace_id=${TRACE_ID}"

# 5. Verify each signal landed.
echo
echo "==> [5/5] Verify signals landed"
sleep 5  # batch processor flush window

# Tempo: trace API takes a few seconds for the trace to be queryable
attempt=0
while [ $attempt -lt 15 ]; do
    body=$(curl -sf "http://localhost:${TEMPO_HTTP_PORT}/api/traces/${TRACE_ID}" 2>/dev/null || true)
    if [ -n "$body" ] && echo "$body" | grep -q "fga.authorize"; then
        echo "    tempo: trace ${TRACE_ID} found and contains fga.authorize"
        break
    fi
    attempt=$((attempt+1))
    sleep 2
done
if [ $attempt -ge 15 ]; then
    echo "FAIL: trace ${TRACE_ID} not found in Tempo after 30s"
    exit 4
fi

# Prometheus: check that the synthetic metric arrived
attempt=0
while [ $attempt -lt 15 ]; do
    resp=$(curl -sf "http://localhost:${PROMETHEUS_PORT}/api/v1/query?query=fga_decisions_total" 2>/dev/null || true)
    if echo "$resp" | grep -q '"resultType":"vector"' && echo "$resp" | grep -q '"value"'; then
        echo "    prometheus: fga_decisions_total present"
        break
    fi
    attempt=$((attempt+1))
    sleep 2
done
if [ $attempt -ge 15 ]; then
    echo "FAIL: fga_decisions_total not present in Prometheus after 30s"
    echo "    last response: $resp"
    exit 4
fi

# Loki: query for our synthetic log line
attempt=0
end_ns=$(date +%s)000000000
start_ns=$((end_ns - 300000000000))
while [ $attempt -lt 15 ]; do
    resp=$(curl -sfG "http://localhost:${LOKI_PORT}/loki/api/v1/query_range" \
        --data-urlencode 'query={service_name="aim-smoke-test"}' \
        --data-urlencode "start=${start_ns}" \
        --data-urlencode "end=${end_ns}" 2>/dev/null || true)
    if echo "$resp" | grep -q "fga.decision"; then
        echo "    loki: fga.decision log line present"
        break
    fi
    attempt=$((attempt+1))
    sleep 2
done
if [ $attempt -ge 15 ]; then
    echo "FAIL: fga.decision log not present in Loki after 30s"
    echo "    last response: $resp"
    exit 4
fi

echo
echo "==> PASS: all three signals landed end-to-end"
echo "    Trace : http://localhost:${GRAFANA_PORT}/explore?left=%7B%22datasource%22:%22tempo%22,%22queries%22:%5B%7B%22queryType%22:%22traceql%22,%22query%22:%22${TRACE_ID}%22%7D%5D%7D"
echo "    Metric: http://localhost:${PROMETHEUS_PORT}/graph?g0.expr=fga_decisions_total"
echo "    Logs  : http://localhost:${GRAFANA_PORT}/explore?left=%7B%22datasource%22:%22loki%22,%22queries%22:%5B%7B%22expr%22:%22%7Bservice_name%3D%5C%22aim-smoke-test%5C%22%7D%22%7D%5D%7D"
