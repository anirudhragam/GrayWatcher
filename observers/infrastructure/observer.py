
"""
Infrastructure Observer for GrayWatcher
Collects pod health metrics from Kubernetes infrastructure perspective
"""
import os
from kubernetes import client, config
from kubernetes.client.rest import ApiException
import logging
import requests
import time

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("INFRA-OBSERVER")

class InfrastructureObserver:
    def __init__(self):
        """Initialize infrastructure observer (Deployment mode - cluster-wide monitoring)"""
        self.observer_id = os.getenv('HOSTNAME', 'infra-observer-unknown')
        self.grace_period_seconds = 120

        verdict_server_url = os.getenv('VERDICT_SERVER_URL', 'http://localhost:80')
        self.observations_url = f"{verdict_server_url}/observations"
        self.interval = int(os.environ.get('INTERVAL', '15')) 

        # Load kubernetes config
        try:
            config.load_incluster_config()
            logger.info("Loaded in-cluster Kubernetes config")
        except Exception as e:
            logger.warning(f"Failed to load in-cluster config: {e}")
            logger.info("Attempting to load local kubeconfig")
            config.load_kube_config()

        # Create Kubernetes API clients
        self.core_api = client.CoreV1Api()
        self.custom_api = client.CustomObjectsApi()
        
        logger.info(f"Infrastructure Observer initialized: {self.observer_id}")
        logger.info(f"Observation interval: {self.interval} seconds")

    def is_within_grace_period(self, pod):
        """Check if the pod is within the grace period after creation"""
        if not pod.status.start_time:
            return True
        
        pod_age_seconds = (time.time() - pod.status.start_time.timestamp())
        return pod_age_seconds < self.grace_period_seconds

    def get_pod_metrics(self, namespace, pod_name):
        """Get pod resource metrics from Kubernetes Metrics Server"""
        try:
            metrics = self.custom_api.get_namespaced_custom_object(
                group="metrics.k8s.io",
                version="v1beta1",
                namespace=namespace,
                plural="pods",
                name=pod_name
            )
            
            # Extract container metrics (use first container)
            if 'containers' in metrics and len(metrics['containers']) > 0:
                container_metrics = metrics['containers'][0]['usage']
                
                # Parse CPU and memory
                cpu_usage = self._parse_cpu(container_metrics.get('cpu', '0'))
                memory_usage = self._parse_memory(container_metrics.get('memory', '0'))
                
                return {
                    'cpu_usage_millicores': cpu_usage,
                    'memory_usage_bytes': memory_usage
                }
            
            return None
            
        except Exception as e:
            logger.debug(f"Failed to get metrics for {namespace}/{pod_name}: {e}")
            return None

    def _parse_cpu(self, cpu_string):
        """Parse CPU string to millicores
        Examples: '125m' → 125, '1' → 1000, '500m' → 500
        """
        cpu_string = str(cpu_string)
        
        # Nanocores
        if cpu_string.endswith('n'):
            return int(cpu_string[:-1]) / 1_000_000
        # Millicores
        elif cpu_string.endswith('m'):
            return int(cpu_string[:-1])
        # Full cores:
        else:
            try:
                return float(cpu_string) * 1000
            except ValueError:
                return 0

    def _parse_memory(self, memory_string):
        """Parse memory string to bytes
        Examples: '64Mi' → 67108864, '128Ki' → 131072, '1Gi' → 1073741824
        """
        memory_string = str(memory_string)
        
        units = {
            'Ki': 1024,
            'Mi': 1024 ** 2,
            'Gi': 1024 ** 3,
            'Ti': 1024 ** 4,
            'K': 1000,
            'M': 1000 ** 2,
            'G': 1000 ** 3,
            'T': 1000 ** 4,
        }
        
        # Convert memory string with suffix to bytes
        for suffix, multiplier in units.items():
            if memory_string.endswith(suffix):
                try:
                    value = int(memory_string[:-len(suffix)])
                    return value * multiplier
                except ValueError:
                    return 0
        
        # If no suffix assume bytes
        try:
            return int(memory_string)
        except ValueError:
            return 0

    def calculate_utilization(self, pod, pod_metrics):
        """Calculate CPU and memory utilization percentages"""
        if not pod_metrics:
            return 0.0, 0.0
        
        # Get requested resources from pod spec
        if not pod.spec.containers or len(pod.spec.containers) == 0:
            return 0.0, 0.0
            
        container = pod.spec.containers[0]
        
        # Get resource requests (with defaults)
        requests = container.resources.requests if container.resources and container.resources.requests else {}
        cpu_request = self._parse_cpu(requests.get('cpu', '100m'))
        memory_request = self._parse_memory(requests.get('memory', '128Mi'))
        
        # Calculate percentages
        cpu_percent = (pod_metrics['cpu_usage_millicores'] / cpu_request) * 100 if cpu_request > 0 else 0
        memory_percent = (pod_metrics['memory_usage_bytes'] / memory_request) * 100 if memory_request > 0 else 0
        
        return round(cpu_percent, 2), round(memory_percent, 2)

    def collect_observations(self):
        """Collect observations for all pods in the cluster"""
        observations = []
        try:
            # Query ALL pods cluster-wide (no field_selector)
            pods = self.core_api.list_pod_for_all_namespaces()
            logger.info(f"Found {len(pods.items)} pods in cluster")

            for pod in pods.items:
                # Skip system namespaces
                if pod.metadata.namespace in ['kube-node-lease', 'kube-public', 'kube-system',
                                               'linkerd', 'linkerd-viz', 'graywatcher',
                                               'gmp-system', 'gmp-public', 'gke-managed-cim',
                                               'chaos-mesh', 'cert-manager']:
                    continue

                obs = self.create_observation(pod)
                observations.append(obs)

                # Log observation info
                pod_name = pod.metadata.name
                namespace = pod.metadata.namespace
                status = obs['status']
                metrics = obs['metrics']
            
                logger.info(
                    f"  Pod: {namespace}/{pod_name} - "
                    f"Status: {status.upper()} "
                    f"Phase: {metrics['phase']}, "
                    f"Containers: {metrics['ready_containers']}/{metrics['total_containers']}, "
                    f"Restarts: {metrics['restart_count']}, "
                    f"CPU: {metrics.get('cpu_utilization_percent', 0):.1f}%, "
                    f"Memory: {metrics.get('memory_utilization_percent', 0):.1f}%"
                )

        except ApiException as e:
            logger.error(f"Kubernetes API error: {e}")
        except Exception as e:
            logger.error(f"Unexpected error collecting observations: {e}")

        return observations
    
    def create_observation(self, pod):
        """Create an observation object for a given pod"""
        pod_name = pod.metadata.name
        namespace = pod.metadata.namespace
        target_id = f'pod/{namespace}/{pod_name}'

        status, metrics = self.analyze_pod_health(pod)
        
        observation = {
            'observer_id': self.observer_id,
            'observer_type': 'infrastructure',
            'target_id': target_id,
            'status': status,
            'metrics': metrics,
            'confidence': 1.0,
            'timestamp': int(time.time() * 1000),
            'metadata': {
                'node': pod.spec.node_name if pod.spec.node_name else 'unknown',
                'namespace': namespace,
                'pod_name': pod_name
            }
        }
        return observation

    def analyze_pod_health(self, pod):
        """Analyze pod health from infrastructure metrics"""
        metrics = {}

        # Get Pod phase
        phase = pod.status.phase
        metrics['phase'] = phase

        # Check container statuses
        ready_containers = 0
        total_containers = 0
        total_restarts = 0

        if pod.status.container_statuses:
            total_containers = len(pod.status.container_statuses)
            for container in pod.status.container_statuses:
                if container.ready:
                    ready_containers += 1
                total_restarts += container.restart_count

        metrics['ready_containers'] = ready_containers
        metrics['total_containers'] = total_containers
        metrics['restart_count'] = total_restarts

        # Check pod conditions
        conditions = {}
        if pod.status.conditions:
            for condition in pod.status.conditions:
                conditions[condition.type] = condition.status == 'True'
        metrics['conditions'] = conditions

        # Get pod resource metrics from Metrics Server
        pod_metrics = self.get_pod_metrics(pod.metadata.namespace, pod.metadata.name)
        
        if pod_metrics:
            metrics['cpu_usage_millicores'] = pod_metrics['cpu_usage_millicores']
            metrics['memory_usage_bytes'] = pod_metrics['memory_usage_bytes']
            
            # Calculate utilization percentages
            cpu_percent, memory_percent = self.calculate_utilization(pod, pod_metrics)
            metrics['cpu_utilization_percent'] = cpu_percent
            metrics['memory_utilization_percent'] = memory_percent
        else:
            # Metrics unavailable
            metrics['cpu_usage_millicores'] = 0
            metrics['memory_usage_bytes'] = 0
            metrics['cpu_utilization_percent'] = 0.0
            metrics['memory_utilization_percent'] = 0.0
            cpu_percent = 0.0
            memory_percent = 0.0

        # Determine overall health status
        status = self.determine_status(
            pod=pod,
            phase=phase,
            ready_containers=ready_containers,
            total_containers=total_containers,
            restart_count=total_restarts,
            conditions=conditions,
            cpu_percent=cpu_percent,
            memory_percent=memory_percent
        )

        return status, metrics
    
    def determine_status(self, pod, phase, ready_containers, total_containers, 
                        restart_count, conditions, cpu_percent, memory_percent):
        """Determine pod health status based on infrastructure metrics
        
        Logic:
            - UNHEALTHY: Pod in Failed or Unknown phase
            - DEGRADED: Pod pending too long, containers not ready, high restarts,
                       conditions failing, high resource utilization (after grace period)
            - HEALTHY: Pod running, all containers ready, low restarts, conditions passing
        """
        
        # Check if within grace period
        within_grace_period = self.is_within_grace_period(pod)

        # No grace period for hard failures
        if phase in ['Failed', 'Unknown']:
            return 'unhealthy'
        
        # Grace period handling
        if within_grace_period:
            # Only flag excessive restarts during grace period
            if restart_count > 5:
                return 'degraded'
            else:
                return 'healthy'
        
        # After grace period, perform all checks
        
        # Pod pending too long
        if phase == 'Pending':
            return 'degraded'
        
        # Not all containers ready
        if total_containers > 0 and ready_containers < total_containers:
            return 'degraded'
        
        # Check resource utilization (NEW)
        if cpu_percent > 90 or memory_percent > 90:
            return 'degraded'
        
        # High restart count
        if restart_count > 5:
            return 'degraded'
        
        # Pod conditions
        if not conditions.get('Ready', False):
            return 'degraded'
        
        if not conditions.get('ContainersReady', False):
            return 'degraded'
        
        # All checks passed
        if phase == 'Running' and ready_containers == total_containers:
            return 'healthy'
        
        # Default to degraded
        return 'degraded'

    def post_observation(self, obs):
        """POST a single observation to the verdict server"""
        try:
            response = requests.post(self.observations_url, json=obs, timeout=5)
            response.raise_for_status()
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to POST observation for {obs.get('target_id')}: {e}")

    def run(self):
        """Main loop to collect and send observations at regular intervals"""
        logger.info("Starting observation loop...")
        
        while True:
            try:
                observations = self.collect_observations()
                logger.info(f"Collected {len(observations)} observations, posting to {self.observations_url}")

                for obs in observations:
                    self.post_observation(obs)
                    
            except Exception as e:
                logger.error(f"Error in observer loop: {e}")

            time.sleep(self.interval)


def main():
    """Main entry point"""
    # No NODE_NAME required anymore - cluster-wide monitoring
    observer = InfrastructureObserver()
    observer.run()


if __name__ == "__main__":
    main()
