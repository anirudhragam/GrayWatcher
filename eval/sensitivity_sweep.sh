#!/usr/bin/env bash
set -euo pipefail

# Latency sensitivity sweep for NetworkChaos on frontend.
# Runs evaluate.sh for each latency value, with a fresh verdict server
# store between runs to avoid window contamination.

START_MS=10
END_MS=100
STEP_MS=10
BASELINE_WAIT=90     # seconds to accumulate clean baseline after restart
TIMEOUT=300          # max seconds to wait for detection per run
INTERVAL=15          # polling interval

RESULTS_DIR="sensitivity_results"
CHAOS_DIR="chaos/sensitivity"
mkdir -p "$RESULTS_DIR" "$CHAOS_DIR"

SUMMARY_FILE="$RESULTS_DIR/summary.csv"
echo "latency_ms,detected,detection_latency_s,grayness,detected_service" > "$SUMMARY_FILE"

echo "========================================"
echo " Latency sensitivity sweep"
echo " Range  : ${START_MS}ms to ${END_MS}ms (step ${STEP_MS}ms)"
echo " Results: $RESULTS_DIR/"
echo "========================================"
echo ""

for latency_ms in $(seq $START_MS $STEP_MS $END_MS); do
    CHAOS_NAME="frontend-latency-${latency_ms}ms"
    CHAOS_FILE="$CHAOS_DIR/network-chaos-${latency_ms}ms.yaml"

    # ── 1. Generate chaos YAML for this latency ──────────────────────────
    cat > "$CHAOS_FILE" <<EOF
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: ${CHAOS_NAME}
  namespace: online-boutique
spec:
  action: delay
  mode: all
  selector:
    namespaces:
      - online-boutique
    labelSelectors:
      app: frontend
  delay:
    latency: "${latency_ms}ms"
  direction: to
  duration: "5m"
EOF

    echo "========================================"
    echo " Run: ${latency_ms}ms latency"
    echo "========================================"

    # ── 2. Clean up any leftover chaos from previous run ─────────────────
    kubectl delete networkchaos --all -n online-boutique 2>/dev/null || true

    # ── 3. Restart verdict server to clear the observation window ─────────
    echo "--> Restarting verdict server to clear observation window..."
    kubectl rollout restart deployment/verdict-server -n graywatcher
    kubectl rollout status deployment/verdict-server -n graywatcher --timeout=60s

    # ── 4. Wait for clean baseline to accumulate ──────────────────────────
    echo "--> Waiting ${BASELINE_WAIT}s for clean baseline..."
    sleep $BASELINE_WAIT

    # ── 5. Apply chaos and poll for detection ─────────────────────────────
    kubectl apply -f "$CHAOS_FILE"
    echo "--> Chaos applied: ${latency_ms}ms"
    CHAOS_START=$(date +%s)

    kubectl port-forward -n graywatcher svc/verdict-server 8080:80 &
    PF_PID=$!
    trap "kill $PF_PID 2>/dev/null; kubectl delete -f $CHAOS_FILE 2>/dev/null || true" EXIT
    sleep 3

    DETECTED=false
    DETECTION_ELAPSED="-"
    GRAYNESS="-"
    DETECTED_SERVICE="-"

    while true; do
        NOW=$(date +%s)
        ELAPSED=$(( NOW - CHAOS_START ))
        RESPONSE=$(curl -s http://localhost:8080/verdicts)

        echo "  [${ELAPSED}s] $(echo "$RESPONSE" | python3 -c "
import sys, json
verdicts = json.load(sys.stdin)
bad = [v for v in verdicts if v.get('verdict_type') in ('gray_failure','hard_failure')]
if bad:
    for v in bad:
        print(f\"{v['target_id']}: {v['verdict_type']} (grayness={v['confidence']:.4f})\")
else:
    print('all healthy')
")"

        if echo "$RESPONSE" | grep -q '"gray_failure"\|"hard_failure"'; then
            DETECTION_ELAPSED=$ELAPSED
            DETECTED=true
            read DETECTED_SERVICE GRAYNESS < <(echo "$RESPONSE" | python3 -c "
import sys, json
verdicts = json.load(sys.stdin)
bad = [v for v in verdicts if v.get('verdict_type') in ('gray_failure','hard_failure')]
if bad:
    v = bad[0]
    print(v['target_id'], v['confidence'])
")
            echo "--> Detected at ${DETECTION_ELAPSED}s (service=${DETECTED_SERVICE}, grayness=${GRAYNESS})"
            break
        fi

        if [ $ELAPSED -ge $TIMEOUT ]; then
            echo "--> Timeout: no fault detected within ${TIMEOUT}s"
            break
        fi

        sleep $INTERVAL
    done

    # ── 6. Clean up ───────────────────────────────────────────────────────
    kill $PF_PID 2>/dev/null || true
    trap - EXIT
    kubectl delete -f "$CHAOS_FILE" 2>/dev/null || true

    # ── 7. Record result ──────────────────────────────────────────────────
    echo "$latency_ms,$DETECTED,$DETECTION_ELAPSED,$GRAYNESS,$DETECTED_SERVICE" >> "$SUMMARY_FILE"
    echo ""
done

echo "========================================"
echo " SENSITIVITY SWEEP COMPLETE"
echo "========================================"
echo ""
cat "$SUMMARY_FILE"
echo ""
echo "Full logs: $RESULTS_DIR/"
