#!/usr/bin/env bash
set -euo pipefail

RUNS=5
DURATION_MINUTES=30
INTERVAL=$(( DURATION_MINUTES * 60 / RUNS ))   # seconds between runs (360s = 6 min)
LOG_DIR="baseline_logs"
mkdir -p "$LOG_DIR"

total_verdicts=0
total_non_healthy=0

echo "==> Baseline false-positive rate test: $RUNS runs over ${DURATION_MINUTES} minutes"
echo "==> Sampling every ${INTERVAL}s"
echo ""

for run in $(seq 1 $RUNS); do
    TIMESTAMP=$(date '+%Y-%m-%dT%H:%M:%S')
    LOG_FILE="$LOG_DIR/run_${run}.json"

    echo "--- Run $run / $RUNS  ($TIMESTAMP) ---"

    # Fresh port-forward each run to avoid dropped connections during long sleeps
    kubectl port-forward -n graywatcher svc/verdict-server 8080:80 &
    PF_PID=$!
    sleep 3

    RESPONSE=$(curl -s http://localhost:8080/verdicts)
    kill $PF_PID 2>/dev/null
    wait $PF_PID 2>/dev/null || true
    echo "$RESPONSE" > "$LOG_FILE"

    # Use Python to parse and print everything; write COUNT: line at the end
    TMPOUT=$(echo "$RESPONSE" | python3 -c "
import sys, json
verdicts = json.load(sys.stdin)
bad = [v for v in verdicts if v.get('verdict_type') != 'healthy']
for v in bad:
    print(f\"  FLAGGED: {v['target_id']} -> {v['verdict_type']} (confidence={v['confidence']:.4f}) | {v['reason']}\")
print(f'COUNT:{len(verdicts)}:{len(bad)}')
")

    # Extract totals from the COUNT: sentinel line
    COUNTS=$(echo "$TMPOUT" | grep '^COUNT:')
    run_total=$(echo "$COUNTS" | cut -d: -f2)
    run_non_healthy_count=$(echo "$COUNTS" | cut -d: -f3)

    # Print flagged lines (everything except the COUNT: line)
    echo "$TMPOUT" | grep -v '^COUNT:' || true

    total_verdicts=$(( total_verdicts + run_total ))
    total_non_healthy=$(( total_non_healthy + run_non_healthy_count ))

    echo "  Verdicts: $run_total  |  Non-healthy: $run_non_healthy_count"

    if [ "$run" -lt "$RUNS" ]; then
        echo "  Waiting ${INTERVAL}s until next sample..."
        echo ""
        sleep "$INTERVAL"
    fi
done

echo ""
echo "===== FALSE POSITIVE RATE SUMMARY ====="
echo "  Total runs       : $RUNS"
echo "  Total verdicts   : $total_verdicts"
echo "  Non-healthy seen : $total_non_healthy"
python3 -c "
total=$total_verdicts
bad=$total_non_healthy
fpr = (bad / total * 100) if total > 0 else 0
print(f'  False positive rate: {bad}/{total} = {fpr:.2f}%')
"
echo "  Raw logs saved to: $LOG_DIR/"
