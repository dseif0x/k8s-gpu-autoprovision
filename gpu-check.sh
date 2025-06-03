#!/bin/bash
set -e

: "${GPU_NODE_NAME:?Environment variable GPU_NODE_NAME is required}"
: "${WAKE_ENDPOINT:?Environment variable WAKE_ENDPOINT is required}"
: "${SHUTDOWN_ENDPOINT:?Environment variable SHUTDOWN_ENDPOINT is required}"

export KUBECONFIG=/tmp/incluster-kubeconfig
TOKEN_PATH=/var/run/secrets/kubernetes.io/serviceaccount/token
CA_PATH=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
SERVER="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}"

cat >"$KUBECONFIG" <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: ${CA_PATH}
    server: ${SERVER}
  name: incluster
contexts:
- context:
    cluster: incluster
    user: default
  name: default
current-context: default
users:
- name: default
  user:
    token: $(cat "$TOKEN_PATH")
EOF

PENDING_GPU_PODS=$(kubectl get pods -A -o json | jq '[.items[] |
  select(
    (.spec.containers[].resources.requests."nvidia.com/gpu"? // 0) > 0
    and (.spec.nodeName == null or .spec.nodeName == "")
  )
] | length')

ACTIVE_GPU_PODS=$(kubectl get pods -A -o json | jq '[.items[] |
  select(
    (.spec.nodeName == "'$GPU_NODE_NAME'")
    and (.spec.containers[].resources.requests."nvidia.com/gpu"? // 0) > 0
  )
] | length')

if (( PENDING_GPU_PODS > 0 )); then
  echo "⏫ Pending GPU pods detected - waking node"
  curl -fsSL -X POST "$WAKE_ENDPOINT"
elif (( ACTIVE_GPU_PODS == 0 )); then
  echo "🔻 No active GPU pods - shutting node down"
  curl -fsSL -X POST "$SHUTDOWN_ENDPOINT"
else
  echo "🟢 GPU busy - nothing to do"
fi