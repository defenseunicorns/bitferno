package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Chart represents the minimal information we need from a Chart.yaml.
type Chart struct {
	Name         string       `yaml:"name"`
	Dependencies []Dependency `yaml:"dependencies"`
}

// Dependency represents a chart dependency.
type Dependency struct {
	Name string `yaml:"name"`
}

func main() {
	// Path to the bitnami charts folder.
	basePath := "/home/corang/bitferno/bitnami"

	// Read all Chart.yaml files
	charts, err := readCharts(basePath)
	if err != nil {
		log.Fatalf("error reading charts: %v", err)
	}

	// Build dependency graph: key is chart name; value: slice of dependent chart names.
	graph := buildGraph(charts)

	// Do a topological sort to get a deploy order
	order, err := topoSort(graph)
	if err != nil {
		log.Fatalf("error during topological sort: %v", err)
	}
	for _, name := range order {
		fmt.Printf("%s\n", name)
	}
}

// readCharts walks the given directory and reads all Chart.yaml files.
func readCharts(root string) (map[string]*Chart, error) {
	charts := make(map[string]*Chart)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Check if this file is Chart.yaml
		if d.IsDir() || d.Name() != "Chart.yaml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var chart Chart
		if err = yaml.Unmarshal(data, &chart); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		if chart.Name == "" {
			return fmt.Errorf("chart at %s missing name", path)
		}

		charts[chart.Name] = &chart
		return nil
	})
	return charts, err
}

// buildGraph creates a dependency graph
// For each chart, for each dependency that exists in charts, we add an edge dependency -> chart.
func buildGraph(charts map[string]*Chart) map[string][]string {
	graph := make(map[string][]string)
	// Initialize graph nodes
	for name := range charts {
		graph[name] = []string{}
	}

	// Add edges: if chart A depends on chart B, then B must be deployed before A.
	for name, chart := range charts {
		for _, dep := range chart.Dependencies {
			// Only consider dependencies that are present in our charts map
			if _, exists := charts[dep.Name]; exists {
				// add edge: dep.Name -> name
				graph[dep.Name] = append(graph[dep.Name], name)
			}
		}
	}
	return graph
}

// topoSort performs a topological sort using Kahn's algorithm.
func topoSort(graph map[string][]string) ([]string, error) {
	// Count incoming edges for each node
	inDegree := make(map[string]int)
	for node := range graph {
		inDegree[node] = 0
	}
	for _, deps := range graph {
		for _, d := range deps {
			inDegree[d]++
		}
	}

	// Collect all nodes with no incoming edges.
	var queue []string
	for node, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, node)
		}
	}

	var order []string
	for len(queue) > 0 {
		// Pop the first element
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// Decrease the degree of all neighbors
		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If the order does not include all nodes, there is a cycle
	if len(order) != len(graph) {
		return nil, fmt.Errorf("cycle detected or missing dependencies, cannot determine deploy order")
	}

	return order, nil
}