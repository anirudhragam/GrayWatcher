#!/usr/bin/env bash
set -euo pipefail

echo "==> Waiting 90s for baseline observations to accumulate..."
sleep 90

echo "==> Baseline verdicts:"
kubectl port-forward -n graywatcher svc/verdict-server 8080:80 &
PF_PID=$!
sleep 2
curl -s http://localhost:8080/verdicts | python3 -m json.tool
kill $PF_PID 2>/dev/null
