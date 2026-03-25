package snow

import (
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	node *snowflake.Node
	mu   sync.Mutex
)

func initSnowId(machineId int) error {
	mu.Lock()
	defer mu.Unlock()

	if node == nil {
		var err error
		node, err = snowflake.NewNode(int64(machineId))
		if err != nil {
			return err
		}
	}

	return nil
}

// 采用懒加载的机制
func GenerateSnowId(machineId int) (string, error) {
	if node == nil {
		if err := initSnowId(machineId); err != nil {
			return "", err
		}
	}

	return node.Generate().String(), nil
}
