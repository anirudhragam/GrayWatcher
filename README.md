# GrayWatcher
GrayWatcher: Kubernetes-Native Gray Failure Detection via Differential Observability

### Commands to build and push infra-observer image to Docker registry
`docker login
cd observers/infrastructure
docker build -t <username>/infra-observer:v1.0 .
docker push <username>/infra-observer:v1.0`

### Commands to build and push verdict-server image to Docker registry
`cd verdict-server
docker build -t <username>/verdict-server:v1.0 .
docker push <username>/verdict-server:v1.0`

### Deploy Graywatcher
`sh deploy_graywatcher.sh`

### Port-forwarding (for local testing)
`kubectl port-forward -n graywatcher svc/verdict-server 8080:80`
