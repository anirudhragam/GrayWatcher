"""
Service-Mesh Observer for GrayWatcher
Collects service latency metrics from Kubernetes service-mesh perspective
"""
import os
import logging
import requests
import time

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

logger = logging.getLogger("MESH-OBSERVER")


class ServiceMeshObserver:
    def __init__(self):
        self.observer_id = "mesh-observer"
        self.observer_type = "mesh"

        self.prometheus_url = os.getenv('PROMETHEUS_URL', 'http://prometheus.linkerd-viz.svc.cluster.local:9090')
        verdict_server_url = os.getenv('VERDICT_SERVER_URL', 'http://localhost:80')
        self.observations_url = f"{verdict_server_url}/observations"

        # Get observation interval (default 15 seconds)
        self.interval = int(os.getenv('INTERVAL', '30'))

        # Log configuration
        logger.info(f"Mesh Observer initialized")
        logger.info(f"Prometheus: {self.prometheus_url}")
        logger.info(f"Verdict Server: {self.observations_url}")
        logger.info(f"Interval: {self.interval}s")

    def get_deployments(self):
        """Get list of deployments with Linkerd metrics"""

        # This query returns all deployments that have inbound traffic
        query = 'sum(response_total{direction="inbound"}) by (namespace, deployment)'

        results = self.query_prometheus(query)
        deployments = []

        for result in results:
            metric = result['metric']
            namespace = metric.get('namespace')
            deployment = metric.get('deployment')
            
            # Skip if missing labels
            if not namespace or not deployment:
                continue
            
            # Skip system namespaces
            if namespace in ['kube-system', 'kube-public', 'kube-node-lease', 'linkerd', 'linkerd-viz', 'graywatcher']:
                continue
            
            deployments.append({
                'namespace': namespace,
                'deployment': deployment
            })
    
        return deployments
        

    def query_prometheus(self, query):
        """Method Execute PromQL query against Prometheus"""
        try:
            response = requests.get(f"{self.prometheus_url}/api/v1/query", params={'query': query})
            response.raise_for_status()
            data = response.json()
            if data['status'] == 'success':
                return data['data']['result']
            else:
                logger.error(f"Prometheus query failed: {data}")
                return []
        except Exception as e:
            logger.error(f"Error querying Prometheus: {e}")
            return []

    def get_pod_metrics(self, namespace, deployment):
        """Get Linkerd metrics for a specific deployment"""
        
        # Query 1: Success Rate
        success_query = f'''
        sum(rate(response_total{{
            namespace="{namespace}",
            deployment="{deployment}",
            classification="success",
            direction="inbound"
        }}[1m]))
        /
        sum(rate(response_total{{
            namespace="{namespace}",
            deployment="{deployment}",
            direction="inbound"
        }}[1m]))
        '''
        
        # Query 2: P99 Latency
        p99_query = f'''
        histogram_quantile(0.99,
        sum(rate(response_latency_ms_bucket{{
            namespace="{namespace}",
            deployment="{deployment}",
            direction="inbound"
        }}[1m])) by (le)
        )
        '''
        
        # Query 3: Request Rate
        request_query = f'''
        sum(rate(response_total{{
            namespace="{namespace}",
            deployment="{deployment}",
            direction="inbound"
        }}[1m]))
        '''
    
        # Execute queries
        success_result = self.query_prometheus(success_query)
        p99_result = self.query_prometheus(p99_query)
        request_result = self.query_prometheus(request_query)
        
        # Extract values
        success_rate = float(success_result[0]['value'][1]) if success_result else 1.0
        p99_latency = float(p99_result[0]['value'][1]) if p99_result else 0.0
        request_rate = float(request_result[0]['value'][1]) if request_result else 0.0
        
        # Calculate error rate (no separate query needed!)
        error_rate = 1.0 - success_rate
        
        # Return metrics dictionary
        return {
            'success_rate': success_rate,
            'p99_latency_ms': p99_latency,
            'error_rate': error_rate,     
            'request_rate': request_rate
        }
    

    def collect_observations(self):
        """Collect observations for all deployments with linkerd proxies"""
        deployments = self.get_deployments()
        logger.info(f"Found {len(deployments)} deployments with Linkerd proxies")

        observations = []
        for dep in deployments:
            namespace = dep['namespace']
            deployment = dep['deployment']
            target_id = f'deployment/{namespace}/{deployment}'

            try:
                # Get metrics for this deployment
                metrics = self.get_pod_metrics(namespace, deployment)

                # Determine status
                status = self.determine_status(metrics)

                # Build observation
                observation = {
                'observer_id': self.observer_id,
                'observer_type': self.observer_type,
                'target_id': target_id,
                'status': status,
                'confidence': 1.0,
                'metrics': metrics,
                'timestamp': int(time.time() * 1000),
                'metadata': {
                    'deployment': deployment,
                    'namespace': namespace
                }
            }
                
                observations.append(observation)
                logger.info(f"  Deployment: {namespace}/{deployment} - Status: {status.upper()}")
            
            except Exception as e:
                logger.error(f"Failed to collect metrics for {namespace}/{deployment}: {e}")

        return observations
    
    def determine_status(self, metrics):
        """Determine health status from metrics"""
        
        success_rate = metrics['success_rate']
        p99_latency = metrics['p99_latency_ms']
        
        # UNHEALTHY: < 90% success OR latency > 5s
        if success_rate < 0.90 or p99_latency > 5000:
            return 'unhealthy'
        
        # DEGRADED: < 99% success OR latency > 1s
        if success_rate < 0.99 or p99_latency > 1000:
            return 'degraded'
        
        # HEALTHY
        return 'healthy'

    def post_observation(self, obs):
        """POST a single observation to the verdict server"""
        try:
            response = requests.post(self.observations_url, json=obs)
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

           
if __name__ == "__main__":
    observer = ServiceMeshObserver()
    observer.run()


    



