#!/usr/bin/env bash
set -euo pipefail

# CPU load sensitivity sweep for StressChaos on frontend.
# Runs a detection poll for each CPU load percentage, with a fresh verdict server
# store between runs to avoid window contamination.

START_LOAD=80
END_LOAD=100
STEP_LOAD=10
BASELINE_WAIT=120     # seconds to accumulate clean baseline after restart
TIMEOUT=300          # max seconds to wait for detection per run
INTERVAL=5          # polling interval
WORKERS=4            # number of CPU stress workers (matches stress-chaos.yaml)

RESULTS_DIR="cpu_sensitivity_results"
CHAOS_DIR="chaos/sensitivity"
mkdir -p "$RESULTS_DIR" "$CHAOS_DIR"

SUMMARY_FILE="$RESULTS_DIR/summary.csv"
echo "cpu_load_percent,detected,detection_latency_s,confidence,verdict_type,detected_service" > "$SUMMARY_FILE"

echo "========================================"
echo " CPU load sensitivity sweep"
echo " Range  : ${START_LOAD}% to ${END_LOAD}% (step ${STEP_LOAD}%)"
echo " Workers: $WORKERS"
echo " Results: $RESULTS_DIR/"
echo "========================================"
echo ""

for load in $(seq $START_LOAD $STEP_LOAD $END_LOAD); do
    CHAOS_NAME="frontend-cpu-stress-${load}pct"
    CHAOS_FILE="$CHAOS_DIR/stress-chaos-${load}pct.yaml"

    # ── 1. Generate chaos YAML for this CPU load ──────────────────────────
    cat > "$CHAOS_FILE" <<EOF
apiVersion: chaos-mesh.org/v1alpha1
kind: StressChaos
metadata:
  name: ${CHAOS_NAME}
  namespace: online-boutique
spec:
  mode: all
  selector:
    namespaces:
      - online-boutique
    labelSelectors:
      app: frontend
  stressors:
    cpu:
      workers: ${WORKERS}
      load: ${load}
  duration: "5m"
EOF

    echo "========================================"
    echo " Run: ${load}% CPU load"
    echo "========================================"

    # ── 2. Clean up any leftover chaos from previous run ─────────────────
    kubectl delete stresschaos --all -n online-boutique 2>/dev/null || true

    # ── 3. Restart verdict server to clear the observation window ─────────
    echo "--> Restarting verdict server to clear observation window..."
    kubectl rollout restart deployment/verdict-server -n graywatcher
    kubectl rollout status deployment/verdict-server -n graywatcher --timeout=60s

    # ── 4. Wait for clean baseline to accumulate ──────────────────────────
    echo "--> Waiting ${BASELINE_WAIT}s for clean baseline..."
    sleep $BASELINE_WAIT

    # ── 5. Apply chaos and poll for detection ─────────────────────────────
    kubectl apply -f "$CHAOS_FILE"
    echo "--> Chaos applied: ${load}% CPU load"
    CHAOS_START=$(date +%s)

    kubectl port-forward -n graywatcher svc/verdict-server 8080:80 &
    PF_PID=$!
    trap "kill $PF_PID 2>/dev/null; kubectl delete -f $CHAOS_FILE 2>/dev/null || true" EXIT
    sleep 3

    DETECTED=false
    DETECTION_ELAPSED="-"
    CONFIDENCE="-"
    VERDICT_TYPE="-"
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
        print(f\"{v['target_id']}: {v['verdict_type']} (confidence={v['confidence']:.4f})\")
else:
    print('all healthy')
")"

        if echo "$RESPONSE" | grep -q '"gray_failure"\|"hard_failure"'; then
            DETECTION_ELAPSED=$ELAPSED
            DETECTED=true
            read DETECTED_SERVICE CONFIDENCE VERDICT_TYPE < <(echo "$RESPONSE" | python3 -c "
import sys, json
verdicts = json.load(sys.stdin)
bad = [v for v in verdicts if v.get('verdict_type') in ('gray_failure','hard_failure')]
if bad:
    v = bad[0]
    print(v['target_id'], v['confidence'], v['verdict_type'])
")
            echo "--> Detected at ${DETECTION_ELAPSED}s (service=${DETECTED_SERVICE}, verdict=${VERDICT_TYPE}, confidence=${CONFIDENCE})"
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
    echo "$load,$DETECTED,$DETECTION_ELAPSED,$CONFIDENCE,$VERDICT_TYPE,$DETECTED_SERVICE" >> "$SUMMARY_FILE"
    echo ""
done

echo "========================================"
echo " CPU SENSITIVITY SWEEP COMPLETE"
echo "========================================"
echo ""
cat "$SUMMARY_FILE"
echo ""
echo "Full logs: $RESULTS_DIR/"
