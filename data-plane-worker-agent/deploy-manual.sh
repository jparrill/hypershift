#!/bin/bash

set -e

NAMESPACE=${NAMESPACE:-"kube-system"}
AGENT_NAME="data-plane-worker-agent"
IMAGE=${IMAGE:-"quay.io/openshift/hypershift:latest"}

echo "Deploying Data Plane Worker Agent to namespace: $NAMESPACE"

# Crear namespace si no existe
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Crear ConfigMap con la configuración
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: $AGENT_NAME-config
  namespace: $NAMESPACE
data:
  config.json: |
    {
      "enabledWorkers": ["sync-pullsecret"],
      "workerConfigs": {
        "sync-pullsecret": {
          "enabled": true,
          "parameters": {
            "kubelet-config-json-path": "/var/lib/kubelet/config.json",
            "global-ps-secret-name": "global-pull-secret",
            "check-interval": "30s"
          }
        }
      },
      "logLevel": "info"
    }
EOF

# Crear ServiceAccount
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: $AGENT_NAME
  namespace: $NAMESPACE
EOF

# Crear ClusterRole
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: $AGENT_NAME
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
EOF

# Crear ClusterRoleBinding
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $AGENT_NAME
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: $AGENT_NAME
subjects:
- kind: ServiceAccount
  name: $AGENT_NAME
  namespace: $NAMESPACE
EOF

# Crear DaemonSet
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: $AGENT_NAME
  namespace: $NAMESPACE
spec:
  selector:
    matchLabels:
      name: $AGENT_NAME
  template:
    metadata:
      labels:
        name: $AGENT_NAME
    spec:
      serviceAccountName: $AGENT_NAME
      containers:
      - name: $AGENT_NAME
        image: $IMAGE
        command: ["/usr/bin/control-plane-operator"]
        args: ["enable-worker-agent", "run", "--config=/etc/data-plane-worker-agent/config.json"]
        volumeMounts:
        - name: agent-config
          mountPath: /etc/data-plane-worker-agent
        - name: kubelet-config
          mountPath: /var/lib/kubelet
        resources:
          requests:
            memory: "50Mi"
            cpu: "40m"
      volumes:
      - name: agent-config
        configMap:
          name: $AGENT_NAME-config
      - name: kubelet-config
        hostPath:
          path: /var/lib/kubelet
          type: Directory
      tolerations:
      - operator: Exists
EOF

echo "Data Plane Worker Agent deployed successfully!"
echo "Check the status with: kubectl get pods -n $NAMESPACE -l name=$AGENT_NAME"
echo "View logs with: kubectl logs -n $NAMESPACE -l name=$AGENT_NAME -f"