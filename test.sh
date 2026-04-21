Post these in sequence. They establish a baseline then inject an anomaly.

Step 1 — Baseline (run this loop to post 5 normal observations of each type):

for i in 1 2 3 4 5; do
  curl -s -o /dev/null -X POST http://localhost:80/observations \
    -H "Content-Type: application/json" \
    -d "{\"observer_id\":\"infra\",\"observer_type\":\"infrastructure\",\"target_id\":\"pod/default/app\",\"status\":\"healthy\",\"confidence\":1.0,\"metrics\":{\"cpu_utilization_percent\":20.0,\"memory_utilization_percent\":30.0,\"phase\":\"Running\"},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"
  curl -s -o /dev/null -X POST http://localhost:80/observations \
    -H "Content-Type: application/json" \
    -d "{\"observer_id\":\"mesh\",\"observer_type\":\"mesh\",\"target_id\":\"deployment/default/app\",\"status\":\"healthy\",\"confidence\":1.0,\"metrics\":{\"success_rate\":0.99,\"error_rate\":0.01,\"p99_latency_ms\":100.0,\"request_rate\":50.0},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"
  sleep 1
done

Step 2 — Gray failure injection (mesh degrades, infra stays healthy):


curl -X POST http://localhost:80/observations \
  -H "Content-Type: application/json" \
  -d "{\"observer_id\":\"infra\",\"observer_type\":\"infrastructure\",\"target_id\":\"pod/default/app\",\"status\":\"healthy\",\"confidence\":1.0,\"metrics\":{\"cpu_utilization_percent\":20.0,\"memory_utilization_percent\":30.0,\"phase\":\"Running\"},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"

curl -X POST http://localhost:80/observations \
  -H "Content-Type: application/json" \
  -d "{\"observer_id\":\"mesh\",\"observer_type\":\"mesh\",\"target_id\":\"deployment/default/app\",\"status\":\"degraded\",\"confidence\":1.0,\"metrics\":{\"success_rate\":0.91,\"error_rate\":0.09,\"p99_latency_ms\":3500.0,\"request_rate\":50.0},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"

Expected: gray_failure with high positive confidence — p99 jumps from 100ms baseline to 3500ms, so mesh anomaly is large; infra is flat so infraAnomaly ≈ 0; grayness = large positive residual.

Step 3 — Correlated degradation (both spike together, should hit the guard):


curl -X POST http://localhost:80/observations \``
  -H "Content-Type: application/json" \
  -d "{\"observer_id\":\"infra\",\"observer_type\":\"infrastructure\",\"target_id\":\"pod/default/app\",\"status\":\"degraded\",\"confidence\":1.0,\"metrics\":{\"cpu_utilization_percent\":95.0,\"memory_utilization_percent\":92.0,\"phase\":\"Running\"},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"

curl -X POST http://localhost:80/observations \
  -H "Content-Type: application/json" \
  -d "{\"observer_id\":\"mesh\",\"observer_type\":\"mesh\",\"target_id\":\"deployment/default/app\",\"status\":\"degraded\",\"confidence\":1.0,\"metrics\":{\"success_rate\":0.85,\"error_rate\":0.15,\"p99_latency_ms\":4000.0,\"request_rate\":50.0},\"timestamp\":$(($(date +%s) * 1000)),\"metadata\":{}}"

Expected: hard_failure from the correlated degradation guard — both CPU and latency spike well above the 2σ threshold.