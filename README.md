# k8s-gpu-autoprovision

A Kubernetes controller that automatically provisions and deprovisions GPU nodes based on workload demand.

## Overview

This controller monitors GPU resource requests in your Kubernetes cluster and automatically scales GPU nodes up or down by calling webhooks to power VMs on/off or add/remove them from the cluster pool.

## Features

- **Automatic GPU Node Scaling**: Scales up nodes when there are pending GPU pods
- **Power Management**: Powers down idle GPU nodes to save costs
- **Webhook Integration**: Uses HTTP webhooks to control node lifecycle
- **Multi-Node Support**: Manages multiple GPU node groups with different configurations
- **Smart Scaling**: Prefers smaller nodes for scale-up and larger nodes for scale-down

## How It Works

1. **Monitoring**: Every 10 seconds, the controller scans all pods for GPU requests
2. **Scale Up**: If pending GPU pods exceed available capacity, it powers on the smallest suitable nodes
3. **Scale Down**: If nodes are completely idle (0 GPU usage), it powers off the largest idle node
4. **Webhook Calls**: Node lifecycle changes are executed via HTTP POST to configured endpoints

## Configuration

The controller is configured via environment variables for each node group:

```bash
# For a node group named "gpu-node-1":
GPU_NODE_1_NAME=gpu-worker-1
GPU_NODE_1_GPU_COUNT=4
GPU_NODE_1_SCALE_UP_ENDPOINT=http://power-api/gpu-worker-1/start
GPU_NODE_1_SCALE_DOWN_ENDPOINT=http://power-api/gpu-worker-1/stop

# For additional nodes, use different prefixes:
GPU_NODE_2_NAME=gpu-worker-2
GPU_NODE_2_GPU_COUNT=8
GPU_NODE_2_SCALE_UP_ENDPOINT=http://power-api/gpu-worker-2/start
GPU_NODE_2_SCALE_DOWN_ENDPOINT=http://power-api/gpu-worker-2/stop
```

## Deployment

### Using Helm

1. Configure your node groups in a new file `values.yaml`:

```yaml
nodeGroups:
  - name: gpu-worker-1
    gpuCount: 4
    scaleUpEndpoint: http://power-api/gpu-worker-1/start
    scaleDownEndpoint: http://power-api/gpu-worker-1/stop
  - name: gpu-worker-2
    gpuCount: 8
    scaleUpEndpoint: http://power-api/gpu-worker-2/start
    scaleDownEndpoint: http://power-api/gpu-worker-2/stop
```

2. Deploy with Helm:

```bash
helm repo add k8s-gpu-autoprovision https://dseif0x.github.io/k8s-gpu-autoprovision/
helm install k8s-gpu-autoprovision --namespace k8s-gpu-autoprovision --create-namespace --values values.yaml k8s-gpu-autoprovision/k8s-gpu-autoprovision
```

### Using Docker

```bash
docker run -d \
  -e GPU_NODE_1_NAME=gpu-worker-1 \
  -e GPU_NODE_1_GPU_COUNT=4 \
  -e GPU_NODE_1_SCALE_UP_ENDPOINT=http://power-api/start \
  -e GPU_NODE_1_SCALE_DOWN_ENDPOINT=http://power-api/stop \
  ghcr.io/dseif0x/k8s-gpu-autoprovision:latest
```

## Requirements

- Kubernetes cluster with RBAC enabled
- GPU nodes with `nvidia.com/gpu` resource labels
- HTTP endpoints that can power nodes on/off
- Appropriate RBAC permissions to list pods cluster-wide

## RBAC Permissions

The controller requires the following permissions:
- `get`, `list`, `watch` on `pods` across all namespaces

## Building

```bash
go build -o k8s-gpu-autoprovision main.go
```

## Docker Build

```bash
docker build -t k8s-gpu-autoprovision .
```

## License

MIT License - see the source code for full license text.