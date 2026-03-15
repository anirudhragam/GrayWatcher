#!/usr/bin/env bash

# To fail fast on any error, unset variables, or failed pipes
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Applying namespace..."
kubectl apply -f "$SCRIPT_DIR/observers/infrastructure/k8s/namespace.yaml"

echo "==> Applying RBAC..."
kubectl apply -f "$SCRIPT_DIR/observers/infrastructure/k8s/rbac.yaml"
kubectl apply -f "$SCRIPT_DIR/observers/service-mesh/k8s/rbac.yaml"
kubectl apply -f "$SCRIPT_DIR/observers/service-mesh/k8s/linkerd-authz.yaml"

echo "==> Deploying verdict-server..."
kubectl apply -f "$SCRIPT_DIR/verdict_server/k8s/service.yaml"
kubectl apply -f "$SCRIPT_DIR/verdict_server/k8s/deployment.yaml"
kubectl rollout status deployment/verdict-server -n graywatcher --timeout=2m

echo "==> Deploying infra-observer..."
kubectl apply -f "$SCRIPT_DIR/observers/infrastructure/k8s/deployment.yaml"
kubectl rollout status deployment/infra-observer -n graywatcher --timeout=2m

echo "==> Deploying mesh-observer..."
kubectl apply -f "$SCRIPT_DIR/observers/service-mesh/k8s/deployment.yaml"
kubectl rollout status deployment/mesh-observer -n graywatcher --timeout=2m

echo "==> Done."
