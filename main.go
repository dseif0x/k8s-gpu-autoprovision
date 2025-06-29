package main

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strconv"
)

type GPUNode struct {
	Name              string // Node name in the cluster
	GpuCount          int    // GPUs provided by this ONE node
	ScaleUpEndpoint   string // Webhook to power the VM on / add to pool
	ScaleDownEndpoint string // Webhook to power the VM off / remove from pool
}

func main() {
	nodes := loadGPUNodes()
	if len(nodes) == 0 {
		panic("❌ No nodes configured")
	}

	cfg, _ := rest.InClusterConfig()
	client, _ := kubernetes.NewForConfig(cfg)

	fmt.Printf("🚀 GPU watcher started with %d managed node(s)\n", len(nodes))

	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		handleScaling(client, nodes)
	}
}

func handleScaling(client *kubernetes.Clientset, nodes []*GPUNode) {
	requested, nodeUsage, pending, err := collectGpuInfo(client)
	if err != nil {
		fmt.Println("❌ Error collecting GPU info:", err)
		return
	}

	// Build map of currently active nodes (those with any usage)
	activeNodes := make(map[string]*GPUNode)
	currentCap := 0
	used := 0

	for _, n := range nodes {
		if usage, ok := nodeUsage[n.Name]; ok {
			activeNodes[n.Name] = n
			currentCap += n.GpuCount
			used += usage
		}
	}

	available := currentCap - used
	fmt.Printf("📊 Requested: %d GPUs | Used: %d | Available: %d | Pending: %d\n", requested, used, available, pending)

	// -------------------- SCALE UP --------------------
	if pending > available {
		needed := pending - available
		fmt.Printf("🔼 Need %d more GPUs to cover pending pods\n", needed)

		// Find inactive nodes and sort by smallest GpuCount first
		var inactive []*GPUNode
		for _, n := range nodes {
			if _, ok := activeNodes[n.Name]; !ok {
				inactive = append(inactive, n)
			}
		}
		sort.Slice(inactive, func(i, j int) bool {
			return inactive[i].GpuCount < inactive[j].GpuCount
		})

		booted := 0
		for _, n := range inactive {
			safePost(n.ScaleUpEndpoint)
			needed -= n.GpuCount
			booted++
			if needed <= 0 {
				break
			}
		}

		if needed > 0 {
			fmt.Printf("⚠️ Still short %d GPUs even after booting %d node(s)\n", needed, booted)
		}

		return
	}

	// -------------------- SCALE DOWN --------------------
	// Only if there is an idle node (usage == 0) that is actually active
	var bestIdle *GPUNode
	for _, n := range nodes {
		if usage, exists := nodeUsage[n.Name]; exists && usage == 0 {
			// Pick largest idle node for maximum power saving
			if bestIdle == nil || n.GpuCount > bestIdle.GpuCount {
				bestIdle = n
			}
		}
	}

	if bestIdle != nil {
		fmt.Printf("🔽 Powering off idle node %s (%d GPUs)\n", bestIdle.Name, bestIdle.GpuCount)
		safePost(bestIdle.ScaleDownEndpoint)
	} else {
		fmt.Println("✅ Capacity matches demand – no action")
	}
}

func collectGpuInfo(client *kubernetes.Clientset) (requested int, usage map[string]int, pending int, err error) {
	usage = map[string]int{}
	pods, err := client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, nil, 0, err
	}

	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			val, ok := c.Resources.Requests["nvidia.com/gpu"]
			if !ok || val.IsZero() {
				continue
			}
			cnt, _ := val.AsInt64()
			requested += int(cnt)

			if pod.Spec.NodeName == "" && pod.Status.Phase == v1.PodPending {
				pending += int(cnt)
			} else if pod.Status.Phase == v1.PodRunning {
				usage[pod.Spec.NodeName] += int(cnt)
			}
		}
	}
	return requested, usage, pending, nil
}

func safePost(url string) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(`{}`))
	if err != nil {
		fmt.Println("❌ POST to", url, "failed:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("✅ POST to %s [%d]\n", url, resp.StatusCode)
}
func loadGPUNodes() []*GPUNode {
	var nodes []*GPUNode
	for _, raw := range os.Environ() {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, _ := parts[0], parts[1]

		if !strings.HasSuffix(key, "_GPU_COUNT") {
			continue
		}

		prefix := strings.TrimSuffix(key, "_GPU_COUNT")

		name := os.Getenv(prefix + "_NAME")
		countStr := os.Getenv(prefix + "_GPU_COUNT")
		up := os.Getenv(prefix + "_SCALE_UP_ENDPOINT")
		down := os.Getenv(prefix + "_SCALE_DOWN_ENDPOINT")

		if countStr == "" || up == "" || down == "" || name == "" {
			fmt.Printf("⚠️ Incomplete config for %s\n", prefix)
			continue
		}

		count, err := strconv.Atoi(countStr)
		if err != nil {
			fmt.Printf("⚠️ Invalid GPU_COUNT for %s: %v\n", prefix, err)
			continue
		}

		nodes = append(nodes, &GPUNode{
			Name:              name,
			GpuCount:          count,
			ScaleUpEndpoint:   up,
			ScaleDownEndpoint: down,
		})
	}
	return nodes
}
