# Data Plane Worker Agent - Usage Example

## Enabling the Worker Agent

To enable the Data Plane Worker Agent in a HostedControlPlane, simply add the following annotation:

```yaml
apiVersion: hypershift.openshift.io/v1beta1
kind: HostedControlPlane
metadata:
  name: my-cluster
  namespace: clusters
  annotations:
    hypershift.openshift.io/enable-data-plane-worker-agent: "true"
    hypershift.openshift.io/data-plane-worker-agent-workers: "sync-pullsecret"
spec:
  # ... rest of configuration
```

## Available Annotations

### `hypershift.openshift.io/enable-data-plane-worker-agent`
- **Value**: `"true"` or `"false"`
- **Description**: Enables or disables the worker-agent
- **Default**: `"false"`

### `hypershift.openshift.io/data-plane-worker-agent-workers`
- **Value**: Comma-separated list of workers to enable
- **Example**: `"sync-pullsecret,other-worker"`
- **Default**: `"sync-pullsecret"`

## Available Workers

### sync-pullsecret
Synchronizes pull secrets from DataPlane to kubelet config.

**Default parameters:**
- `kubelet-config-json-path`: `/var/lib/kubelet/config.json`
- `global-ps-secret-name`: `global-pull-secret`
- `check-interval`: `30s`

## Behavior

When the worker-agent is enabled:

1. **Automatically creates** a DaemonSet in the DataPlane
2. **Configures** a ConfigMap with worker configuration
3. **Sets up** necessary permissions (ServiceAccount, ClusterRole, ClusterRoleBinding)
4. **Runs** the agent on each DataPlane node

## Migration from Legacy System

The system is backward compatible. If the annotation is not specified, the legacy direct DaemonSet system is used.

## Verification

To verify that the worker-agent is working:

```bash
# Check the DaemonSet
kubectl get daemonset -n kube-system data-plane-worker-agent

# Check the pods
kubectl get pods -n kube-system -l name=data-plane-worker-agent

# Check the configuration
kubectl get configmap -n kube-system data-plane-worker-agent-config -o yaml
```