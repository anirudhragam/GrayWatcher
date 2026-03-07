# GrayWatcher
GrayWatcher: Kubernetes-Native Gray Failure Detection via Differential Observability


### Installing Kubernetes Metrics Server
`kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

kubectl patch deployment metrics-server -n kube-system --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/containers/0/args/-",
    "value": "--kubelet-insecure-tls"
  }
]'`

The kubectl patch command is used to disable TLS verification as the metrics-server cannont verify TLS certificate for Docker Desktop

### Commands to build and push infra-observer image to Docker registry
`docker login
cd observers/infrastructure
docker build -t <username>/infra-observer:v1.0 .
docker push <username>/infra-observer:v1.0`

### Commands to build and push mesh-observer image to Docker registry
`cd observers/service-mesh
docker build -t <username>/mesh-observer:v1.0 .
docker push <username>/mesh-observer:v1.0`

### Commands to build and push verdict-server image to Docker registry
`cd verdict-server
docker build -t <username>/verdict-server:v1.0 .
docker push <username>/verdict-server:v1.0`

### Deploy Graywatcher
`sh deploy_graywatcher.sh`

### Port-forwarding (for local testing)
`kubectl port-forward -n graywatcher svc/verdict-server 8080:80`

## Linkerd

### Linkerd installation
```
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | sh
export PATH=$PATH:$HOME/.linkerd2/bin

# Install Custom Resource Definitions (CRDs)
linkerd install --crds | kubectl apply -f -

# Install Linkerd control plane
linkerd install --set proxyInit.runAsRoot=true | kubectl apply -f -

# Verify installation
linkerd check

# Install Viz extension (includes Prometheus, dashboard, etc.)
linkerd viz install | kubectl apply -f -

# Verify Viz installation
linkerd viz check
```

### Inject Linkerd into Deployments
```
# Inject into existing deployment
kubectl get deployment <deployment-name> -n <namespace> -o yaml | \
  linkerd inject - | \
  kubectl apply -f -

# Verify injection (each pod should have 2 containers)
kubectl get pods -n <namespace>
```

