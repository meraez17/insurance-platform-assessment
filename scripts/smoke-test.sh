#!/usr/bin/env bash
set -euo pipefail

for attempt in $(seq 1 60); do
  if curl -fsS http://localhost:8080/health >/dev/null && curl -fsS http://localhost:8081/health >/dev/null && curl -fsS http://localhost:8082/health >/dev/null; then
    break
  fi
  if [ "$attempt" -eq 60 ]; then echo "services did not become healthy" >&2; exit 1; fi
  sleep 2
done

response=$(curl -fsS -X POST http://localhost:8080/orders -H 'Content-Type: application/json' -H 'Idempotency-Key: smoke-001' -d '{"plate":"ABC123"}')
order_id=$(node -e 'const v=JSON.parse(process.argv[1]); if(!v.id) process.exit(1); process.stdout.write(v.id)' "$response")
replayed=$(curl -fsS -X POST http://localhost:8080/orders -H 'Content-Type: application/json' -H 'Idempotency-Key: smoke-001' -d '{"plate":"ABC123"}')
node -e 'const [a,b]=process.argv.slice(1).map(JSON.parse); if(a.id!==b.id) process.exit(1)' "$response" "$replayed"
echo "smoke test passed for $order_id"
