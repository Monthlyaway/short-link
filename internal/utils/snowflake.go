package utils

import (
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	once sync.Once
)

// InitSnowflake initializes the snowflake node with datacenter and worker IDs
func InitSnowflake(datacenterID, workerID int64) error {
	var err error
	once.Do(func() {
		// Combine datacenter ID and worker ID into a single node ID
		// DatacenterID uses 5 bits (0-31), WorkerID uses 5 bits (0-31)
		nodeID := (datacenterID << 5) | workerID

		node, err = snowflake.NewNode(nodeID)
	})
	return err
}

// GenerateID generates a unique snowflake ID
func GenerateID() (int64, error) {
	if node == nil {
		return 0, fmt.Errorf("snowflake node not initialized")
	}
	return node.Generate().Int64(), nil
}

// GetNode returns the snowflake node instance
func GetNode() *snowflake.Node {
	return node
}
