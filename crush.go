package main

import (
	"fmt"
	"hash/fnv"
	"sort"
)

// Node represents a storage node in the distributed system.
type Node struct {
	ID     int
	Weight int
}

// DataObject represents a data object to be stored.
type DataObject struct {
	ID       int
	Size     int
	Replicas int
}

// CRUSH maps a data object to a set of storage nodes using the Ceph CRUSH algorithm.
func CRUSH(dataObject DataObject, nodes []Node) []Node {
	// Step 1: Generate a hash value for the data object ID.
	hash := fnv.New32a()
	hash.Write([]byte(fmt.Sprintf("%d", dataObject.ID)))
	hashValue := hash.Sum32()

	// Step 2: Sort the nodes by their ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Step 3: Calculate the total weight of all nodes.
	totalWeight := 0
	for _, node := range nodes {
		totalWeight += node.Weight
	}

	// Step 4: Determine the number of replicas to create.
	numReplicas := dataObject.Replicas
	if numReplicas > len(nodes) {
		numReplicas = len(nodes)
	}

	// Step 5: Map the data object to a set of storage nodes using the Ceph CRUSH algorithm.
	result := make([]Node, 0, numReplicas)
	for i := 0; i < numReplicas; i++ {
		// Calculate the index of the node using the hash value, the total weight, and a Ceph-specific pseudorandom function.
		index := int((uint64(hashValue) + uint64(i)*uint64(totalWeight)) % uint64(len(nodes)))
		result = append(result, nodes[index])
	}

	return result
}

func main() {
	// Example usage: map a data object to storage nodes.
	nodes := []Node{
		{ID: 1, Weight: 100},
		{ID: 2, Weight: 200},
		{ID: 3, Weight: 300},
		{ID: 4, Weight: 400},
	}
	dataObject := DataObject{ID: 123, Size: 1024, Replicas: 3}
	mappedNodes := CRUSH(dataObject, nodes)
	fmt.Println("Mapped nodes:", mappedNodes)
}
