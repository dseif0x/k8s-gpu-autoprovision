package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strconv"
)

type NodeGroup struct {
	Name              string
	GpuCount          int
	ScaleUpEndpoint   string
	ScaleDownEndpoint string
}

func main() {
	groups := loadNodeGroups()
	if len(groups) == 0 {
		panic("❌ No node groups configured")
	}

	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	fmt.Println("🚀 GPU watcher started with", len(groups), "node groups")
	watcher, err := clientset.CoreV1().Pods("").Watch(context.TODO(), metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		panic(err)
	}

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
		if !ok || !isGpuPod(pod) {
			continue
		}

		for _, group := range groups {
			go handleScaling(clientset, group)
		}
	}
}

func handleScaling(clientset *kubernetes.Clientset, group *NodeGroup) {
	totalGPU, err := countRequestedGPU(clientset)
	if err != nil {
		fmt.Println("Error counting GPUs:", err)
		return
	}

	if totalGPU >= group.GpuCount {
		fmt.Println("🔼 Requesting scale UP for", group.Name)
		safePost(group.ScaleUpEndpoint)
	} else {
		fmt.Println("🔽 Requesting scale DOWN for", group.Name)
		safePost(group.ScaleDownEndpoint)
	}
}

func countRequestedGPU(clientset *kubernetes.Clientset) (int, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	total := 0
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			if val, ok := c.Resources.Requests["nvidia.com/gpu"]; ok {
				count, _ := val.AsInt64()
				total += int(count)
			}
		}
	}
	return total, nil
}

func isGpuPod(pod *v1.Pod) bool {
	for _, c := range pod.Spec.Containers {
		if val, ok := c.Resources.Requests["nvidia.com/gpu"]; ok && !val.IsZero() {
			return true
		}
	}
	return false
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

func loadNodeGroups() []*NodeGroup {
	var groups []*NodeGroup
	for _, env := range os.Environ() {
		if strings.HasSuffix(env, "_GPU_COUNT") {
			prefix := strings.TrimSuffix(strings.SplitN(env, "=", 2)[0], "_GPU_COUNT")
			countStr := os.Getenv(prefix + "_GPU_COUNT")
			up := os.Getenv(prefix + "_SCALE_UP_ENDPOINT")
			down := os.Getenv(prefix + "_SCALE_DOWN_ENDPOINT")

			if countStr == "" || up == "" || down == "" {
				fmt.Println("⚠️ Incomplete config for", prefix)
				continue
			}
			count, err := strconv.Atoi(countStr)
			if err != nil {
				fmt.Println("⚠️ Invalid GPU_COUNT for", prefix)
				continue
			}
			groups = append(groups, &NodeGroup{
				Name:              prefix,
				GpuCount:          count,
				ScaleUpEndpoint:   up,
				ScaleDownEndpoint: down,
			})
		}
	}
	return groups
}
