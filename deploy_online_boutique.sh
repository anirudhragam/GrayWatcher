#!/usr/bin/env bash
set -euo pipefail

export PATH=$PATH:$HOME/.linkerd2/bin

echo "==> Creating online-boutique namespace..."
kubectl create namespace online-boutique --dry-run=client -o yaml | kubectl apply -f -

echo "==> Deploying Online Boutique..."
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/microservices-demo/main/release/kubernetes-manifests.yaml -n online-boutique

echo "==> Enabling Linkerd injection on namespace..."
kubectl annotate namespace online-boutique linkerd.io/inject=enabled --overwrite

echo "==> Opting loadgenerator out of Linkerd injection..."
kubectl patch deployment loadgenerator -n online-boutique --type=json \
  -p='[{"op":"add","path":"/spec/template/metadata/annotations/linkerd.io~1inject","value":"disabled"}]'

echo "==> Restarting deployments to trigger injection..."
kubectl rollout restart deployment -n online-boutique

echo "==> Waiting for rollout..."
for dep in frontend productcatalogservice cartservice checkoutservice recommendationservice paymentservice emailservice shippingservice currencyservice adservice; do
  kubectl rollout status deployment/$dep -n online-boutique --timeout=5m
done

echo "==> Online Boutique pods:"
kubectl get pods -n online-boutique
echo ""
