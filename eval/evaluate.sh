#!/usr/bin/env bash
set -euo pipefail

CHAOS_FILE=${1:?"Usage: $0 <chaos-yaml-file>"}
TIMEOUT=300
INTERVAL=15

kubectl apply -f "$CHAOS_FILE"
echo "==> Chaos applied: $CHAOS_FILE"
START=$(date +%s)

kubectl port-forward -n graywatcher svc/verdict-server 8080:80 &
PF_PID=$!
trap "kill $PF_PID 2>/dev/null" EXIT
sleep 2

DETECTED=false
while true; do
  NOW=$(date +%s)
  ELAPSED=$((NOW - START))
  RESPONSE=$(curl -s http://localhost:8080/verdicts)
  echo "[$ELAPSED s] $RESPONSE"

  if echo "$RESPONSE" | grep -q '"gray_failure"\|"hard_failure"'; then
    echo "==> Fault detected at ${ELAPSED}s"
    DETECTED=true
    break
  fi

  if [ $ELAPSED -ge $TIMEOUT ]; then
    echo "==> Timeout: no fault detected within ${TIMEOUT}s"
    break
  fi

  sleep $INTERVAL
done

echo "==> Deleting chaos..."
kubectl delete -f "$CHAOS_FILE"

if $DETECTED; then
  echo "==> Waiting for recovery..."
  RECOVER_START=$(date +%s)
  while true; do
    sleep $INTERVAL
    ELAPSED=$(( $(date +%s) - RECOVER_START ))
    RESPONSE=$(curl -s http://localhost:8080/verdicts)
    if echo "$RESPONSE" | grep -q '"healthy"' && ! echo "$RESPONSE" | grep -q '"gray_failure"\|"hard_failure"'; then
      echo "==> Recovered at ${ELAPSED}s after chaos deleted"
      break
    fi
    [ $ELAPSED -ge $TIMEOUT ] && echo "==> Recovery timeout" && break
  done
fi
