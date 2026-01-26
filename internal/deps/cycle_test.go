package deps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCycle_NoCycle(t *testing.T) {
	existing := map[int64][]int64{
		1: {2},
		2: {3},
	}
	newEdges := []Edge{{From: 3, To: 4}}
	err := DetectCycle(existing, newEdges, 0)
	assert.NoError(t, err)
}

func TestDetectCycle_SelfDependency(t *testing.T) {
	existing := map[int64][]int64{}
	newEdges := []Edge{{From: 1, To: 1}}
	err := DetectCycle(existing, newEdges, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on itself")
}

func TestDetectCycle_ExistingCycle(t *testing.T) {
	existing := map[int64][]int64{
		1: {2},
		2: {3},
		3: {1},
	}
	newEdges := []Edge{}
	err := DetectCycle(existing, newEdges, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestDetectCycle_NewEdgeCreatesCycle(t *testing.T) {
	existing := map[int64][]int64{
		1: {2},
		2: {3},
	}
	newEdges := []Edge{{From: 3, To: 1}}
	err := DetectCycle(existing, newEdges, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

func TestValidateNewDependencies_DuplicateDep(t *testing.T) {
	existing := map[int64][]int64{}
	err := ValidateNewDependencies(existing, []int64{1, 1}, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate dependency")
}

func TestValidateNewDependencies_SelfDep(t *testing.T) {
	existing := map[int64][]int64{}
	err := ValidateNewDependencies(existing, []int64{2}, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on itself")
}

func TestValidateNewDependencies_ValidDeps(t *testing.T) {
	existing := map[int64][]int64{
		1: {2},
	}
	err := ValidateNewDependencies(existing, []int64{3}, 4)
	assert.NoError(t, err)
}

func TestValidateNewDependencies_CreatesCycle(t *testing.T) {
	existing := map[int64][]int64{
		1: {2},
		2: {3},
	}
	err := ValidateNewDependencies(existing, []int64{1}, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}
