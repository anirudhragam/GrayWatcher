# GrayWatcher
GrayWatcher: Kubernetes-Native Gray Failure Detection via Differential Observability

---

## Prerequisites

- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) (`gcloud`)
- `kubectl`
- `helm`
- `docker` with [buildx](https://docs.docker.com/buildx/working-with-buildx/)
- [Linkerd CLI](https://linkerd.io/2/getting-started/)

---

## Step 1 — Create a GKE Standard Cluster

```bash
gcloud components install gke-gcloud-auth-plugin

gcloud container clusters create graywatcher-cluster \
  --zone us-west1-a \
  --num-nodes 3 \
  --machine-type e2-standard-4 \
  --disk-type pd-standard \
  --disk-size 50 \
  --project <your-project-id>

# Connect kubectl to the cluster
gcloud container clusters get-credentials graywatcher-cluster \
  --zone us-west1-a \
  --project <your-project-id>

# Verify
kubectl get nodes
```

> **Note:** Use a Standard cluster, not Autopilot. Autopilot blocks the `NET_ADMIN` capability required by Linkerd's init container.

---

## Step 2 — Install Linkerd

```bash
# Download Linkerd CLI
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | sh
export PATH=$PATH:$HOME/.linkerd2/bin

# Add to shell profile to persist across sessions
echo 'export PATH=$PATH:$HOME/.linkerd2/bin' >> ~/.zshrc

# Install Gateway API CRDs (required by Linkerd)
kubectl apply --server-side -f \
  https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml

# Install Linkerd CRDs
linkerd install --crds | kubectl apply -f -

# Install Linkerd control plane
linkerd install --set proxyInit.runAsRoot=true | kubectl apply -f -

# Verify
linkerd check

# Install Viz extension (includes Prometheus)
linkerd viz install | kubectl apply -f -

# Verify
linkerd viz check
```

> **Note:** If `linkerd check` fails with "cluster networks contains all services", run:
> ```bash
> linkerd upgrade --set clusterNetworks="10.0.0.0/8\,100.64.0.0/10\,172.16.0.0/12\,192.168.0.0/16\,34.118.0.0/16\,fd00::/8" | kubectl apply -f -
> ```

---

## Step 3 — Install Kubernetes Metrics Server

```bash
kubectl apply -f \
  https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Verify
kubectl top nodes
```

---

## Step 4 — Build and Push Docker Images

Images must be built for `linux/amd64` (GKE nodes are AMD64). The `--provenance=false` flag is required to avoid attestation manifests that GKE's containerd cannot handle.

```bash
docker login

# Infra observer
cd observers/infrastructure
docker buildx build --platform linux/amd64 --provenance=false \
  -t vera5660/infra-observer:v1.3 --push .
cd ../..

# Mesh observer
cd observers/service-mesh
docker buildx build --platform linux/amd64 --provenance=false \
  -t <username>/mesh-observer:v1.0 --push .
cd ../..

# Verdict server
cd verdict_server
docker buildx build --platform linux/amd64 --provenance=false \
  -t <username>/verdict-server:v1.2 --push .
cd ..
```

---

## Step 5 — Deploy GrayWatcher

```bash
sh deploy_graywatcher.sh

# Verify all pods are running
kubectl get pods -n graywatcher
```

---

## Step 6 — Deploy Online Boutique

```bash
sh deploy_online_boutique.sh

# Verify all pods are running with 2 containers each (app + linkerd-proxy)
kubectl get pods -n online-boutique
```

---

## Step 7 — Install Chaos Mesh

```bash
helm repo add chaos-mesh https://charts.chaos-mesh.org
helm repo update
kubectl create namespace chaos-mesh --dry-run=client -o yaml | kubectl apply -f -

helm install chaos-mesh chaos-mesh/chaos-mesh \
  -n chaos-mesh \
  --set chaosDaemon.runtime=containerd \
  --set chaosDaemon.socketPath=/run/containerd/containerd.sock

# Verify
kubectl get pods -n chaos-mesh
```

> **Note:** GKE uses `containerd`, not Docker. Use the flags above, not the ones in `install_chaosmesh.sh` (which targets Docker Desktop).

---

## Step 8 — Run the Evaluation

### Baseline (verify no false positives)

```bash
cd eval
sh baseline.sh
```

### Chaos experiments

```bash
cd eval

# Network chaos → expect gray_failure on frontend (~32s detection)
sh evaluate.sh chaos/network-chaos.yaml

# Stress chaos → expect gray_failure on frontend (~123s detection)
sh evaluate.sh chaos/stress-chaos.yaml

# Pod chaos → expect gray_failure cascade on frontend (~93s detection)
sh evaluate.sh chaos/pod-chaos.yaml
```

### Sensitivity sweep (latency 10ms → 100ms)

```bash
cd eval
bash sensitivity_sweep.sh
# Results written to sensitivity_results/summary.csv
```

### False positive rate (30-minute baseline)

```bash
cd eval
bash baseline_fpr.sh
# Results written to baseline_logs/
```

---

## Linkerd Authorization (troubleshooting)

If mesh-observer cannot reach Prometheus:

```bash
kubectl apply -f observers/service-mesh/k8s/linkerd-authz.yaml
```

## Port-forwarding (local access)

```bash
kubectl port-forward -n graywatcher svc/verdict-server 8080:80
curl -s http://localhost:8080/verdicts | python3 -m json.tool
```
