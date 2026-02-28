
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
    def __init__(self, node_name):
        self.node_name = node_name
        self.observer_id = f"infra-{node_name}-observer"
        self.grace_period_seconds = 120

        verdict_server_url = os.getenv('VERDICT_SERVER_URL', 'http://localhost:80')
        self.observations_url = f"{verdict_server_url}/observations"

        # Load kubernetes config
        try:
            config.load_incluster_config()
            logger.info("Loaded in-cluster Kubernetes config")
        except Exception as e:
            logger.warning(f"Failed to load in-cluster config: {e}")
            logger.info("Attempting to load local kubeconfig")
            config.load_kube_config()

        # Create Kubernetes API client
        self.v1 = client.CoreV1Api()
        logger.info(f"Infrastructure Observer initialized for node: {self.node_name}")

    def is_within_grace_period(self, pod):
        """Check if the pod is within the grace period after creation"""
        # If no start time available, assume the pod is new
        if not pod.status.start_time:
            return True
        
        # Calculate pod_age
        pod_age_seconds = (time.time() - pod.status.start_time.timestamp())

        return pod_age_seconds < self.grace_period_seconds


    def collect_observations(self):
        """Collect observations for all pods on the node"""
        observations = []
        try:
            pods = self.v1.list_pod_for_all_namespaces(field_selector=f'spec.nodeName={self.node_name}')
            logger.info(f"Found {len(pods.items)} pods on node {self.node_name}")

            for pod in pods.items:
                # Skip system pods. 
                #TODO: Add graywatcher namespace to this list
                if pod.metadata.namespace in ['kube-node-lease', 'kube-public', 'kube-system']:
                    continue

                obs = self.create_observation(pod)
                observations.append(obs)

                # Log detailed observation info
                pod_name = pod.metadata.name
                namespace = pod.metadata.namespace
                status = obs['status']
                metrics = obs['metrics']
            
                logger.info(
                    f"  Pod: {namespace}/{pod_name} - "
                    f"Status: {status.upper()} "
                    f"Phase: {metrics['phase']}, "
                    f"Containers: {metrics['ready_containers']}/{metrics['total_containers']}, "
                    f"Restarts: {metrics['restart_count']}"
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
        'confidence': 1.0, # Hardcoded for now
        'timestamp': int(time.time() * 1000),
        'metadata': {
            'node': self.node_name,
            'namespace': namespace,
            'pod_name': pod_name
        }
    }
        return observation
    

    def analyze_pod_health(self, pod):
        """Analyze pod health from infrastructure metrics."""
        metrics = {}

        # Get Pod phase (Pending, Running, Succeeded, Failed, Unknown)
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

        # Check pod conditions (Ready, Initialized, ContainersReady, PodScheduled)
        conditions = {}
        if pod.status.conditions:
            for condition in pod.status.conditions:
                conditions[condition.type] = condition.status == 'True'

        metrics['conditions'] = conditions

        # Determine overall health status
        status = self.determine_status(
            pod=pod,
            phase=phase,
            ready_containers=ready_containers,
            total_containers=total_containers,
            restart_count=total_restarts,
            conditions=conditions
        )

        return status, metrics
    
    def determine_status(self, pod, phase, ready_containers, total_containers, restart_count, conditions):
        """Function to map infrastructure metrics to overall health status
           Logic:
                - UNHEALTHY: Pod failed, unknown state, or critical infrastructure failure
                - DEGRADED: Pod pending, containers not ready, high restarts, conditions failing (after grace period)
                - HEALTHY: Pod running, all containers ready, low restarts, conditions passing"""
        
        # Check if within grace period
        within_grace_period = False
        if pod is not None:
            within_grace_period = self.is_within_grace_period(pod)

        # No grace period for hard failures
        if phase in ['Failed', 'Unknown']:
            return 'unhealthy'
        
        if within_grace_period:
            # High number of restarts during grace period can also indicate degradation
            if restart_count > 5:
                return 'degraded'
            
            else:
                return 'healthy'
            
        #Pod is pending (after grace period)
        if phase == 'Pending':
            return 'degraded'
        
        #Not all containers are ready (after grace period)
        if total_containers > 0 and ready_containers < total_containers:
            return 'degraded'
        
        # High restart count
        if restart_count > 5:
            return 'degraded'
        
        if not conditions.get('Ready', False):
            return 'degraded'
        
        if not conditions.get('ContainersReady', False):
            return 'degraded'
        
        if phase == 'Running' and ready_containers == total_containers:
            return 'healthy'
        
        # Default to degraded
        return 'degraded'
    
    def _calculate_confidence(self, pod, conditions):
        """Calculate confidence score for observation"""
        within_grace_period = self._is_within_grace_period(pod)
        
        if within_grace_period:
            return 0.3  # Low confidence during startup
        
        confidence = 0.6  # Base
        
        if pod.status.container_statuses:
            confidence += 0.15
        
        if conditions:
            confidence += 0.15
        
        if pod.status.start_time:
            age_seconds = (time.time() - pod.status.start_time.timestamp())
            if age_seconds > 300:  # 5+ minutes old
                confidence += 0.1
        
        return min(confidence, 1.0)
                

    def post_observation(self, obs):
        """POST a single observation to the verdict server"""
        try:
            response = requests.post(self.observations_url, json=obs, timeout=5)
            response.raise_for_status()
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to POST observation for {obs.get('target_id')}: {e}")

    def run(self, interval=15):
        """Main loop to collect and send observations at regular intervals"""
        while True:
            try:
                observations = self.collect_observations()
                logger.info(f"Collected {len(observations)} observations, posting to {self.observations_url}")

                for obs in observations:
                    self.post_observation(obs)
            except Exception as e:
                logger.error(f"Error in observer loop: {e}")

            time.sleep(interval)

def main():
    node_name = os.getenv('NODE_NAME')
    if not node_name:
        logger.error("NODE_NAME environment variable is not set")
        exit(1)

    # Create and run observer
    observer = InfrastructureObserver(node_name)
    observer.run(interval=15)


if __name__ == "__main__":
   main()