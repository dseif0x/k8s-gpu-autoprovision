#!/bin/bash
set -e

: "${GPU_NODE_NAME:?Environment variable GPU_NODE_NAME is required}"
: "${WAKE_ENDPOINT:?Environment variable WAKE_ENDPOINT is required}"
: "${SHUTDOWN_ENDPOINT:?Environment variable SHUTDOWN_ENDPOINT is required}"

# Check for pending GPU pods
PENDING_GPU_PODS=$(kubectl get pods --all-namespaces -o json |
  jq '[.items[] | select(
    (.spec.containers[].resources.requests."nvidia.com/gpu"? // 0) > 0
    and .status.phase == "Pending"
    and (.status.conditions[]? | select(.type == "PodScheduled" and .status == "False"))
  )] | length')

# Check for active GPU pods on the node
ACTIVE_GPU_PODS=$(kubectl get pods --all-namespaces -o json |
  jq '[.items[] | select(
    (.spec.nodeName == "'$GPU_NODE_NAME'")
    and (.spec.containers[].resources.requests."nvidia.com/gpu"? // 0) > 0
    and .status.phase == "Running"
  )] | length')

if [[ "$PENDING_GPU_PODS" -gt 0 ]]; then
  echo "⏫ Found pending GPU pod(s). Waking GPU node..."
  curl -X POST "$WAKE_ENDPOINT"
elif [[ "$ACTIVE_GPU_PODS" -eq 0 ]]; then
  echo "🔻 No active GPU pods. Shutting down GPU node..."
  curl -X POST "$SHUTDOWN_ENDPOINT"
else
  echo "🟢 GPU is busy. No action taken."
fi
