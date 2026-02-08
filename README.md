# GrayWatcher
GrayWatcher: Kubernetes-Native Gray Failure Detection via Differential Observability

# Commands to build and push infra-observer image to Docker registry
1. docker login
2. cd observers/infrastructure
2. docker build -t <docker_username>/infra-observer:v1.0 .
3. docker push <docker_username>/infra-observer:v1.0
