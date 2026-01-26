package deps

import "fmt"

type Edge struct {
	From int64
	To   int64
}

func DetectCycle(existingEdges map[int64][]int64, newEdges []Edge, newTaskID int64) error {
	for _, e := range newEdges {
		if e.From == e.To {
			return fmt.Errorf("task cannot depend on itself: %d", e.From)
		}
	}

	existingEdgesCopy := make(map[int64][]int64)
	for k, v := range existingEdges {
		edges := make([]int64, len(v))
		copy(edges, v)
		existingEdgesCopy[k] = edges
	}

	for _, e := range newEdges {
		existingEdgesCopy[e.From] = append(existingEdgesCopy[e.From], e.To)
	}

	visited := make(map[int64]int)
	var dfs func(node int64) bool
	dfs = func(node int64) bool {
		visited[node] = 1
		for _, neighbor := range existingEdgesCopy[node] {
			if neighbor == newTaskID {
				return true
			}
			if visited[neighbor] == 1 {
				return true
			}
			if visited[neighbor] == 0 {
				if dfs(neighbor) {
					return true
				}
			}
		}
		visited[node] = 2
		return false
	}

	for from := range existingEdgesCopy {
		if visited[from] == 0 {
			if dfs(from) {
				return fmt.Errorf("cycle detected")
			}
		}
	}

	return nil
}

func ValidateNewDependencies(existingEdges map[int64][]int64, newDeps []int64, taskID int64) error {
	for _, dep := range newDeps {
		if dep == taskID {
			return fmt.Errorf("task cannot depend on itself: %d", taskID)
		}
	}

	depCount := make(map[int64]int)
	for _, dep := range newDeps {
		depCount[dep]++
		if depCount[dep] > 1 {
			return fmt.Errorf("duplicate dependency: %d", dep)
		}
	}

	newEdges := make([]Edge, len(newDeps))
	for i, dep := range newDeps {
		newEdges[i] = Edge{From: taskID, To: dep}
	}

	return DetectCycle(existingEdges, newEdges, taskID)
}
