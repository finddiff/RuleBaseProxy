package constant

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
)

var (
	Node *snowflake.Node
)

func InitSequences() {
	node, err := snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
		return
	}
	Node = node
}
