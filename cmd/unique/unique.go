package main

import (
	"encoding/json"
	"fmt"
	"gossip_gloomers/pkg"
	"log"
)

type UniqueNode struct {
	n pkg.INode
}

func main() {
	node := UniqueNode{
		n: pkg.NewNode(),
	}

	node.n.Handle("generate", func(msg pkg.Message) error {
		var body UniqueBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal unique body")
		}

		body.Type = "generate_ok"
		body.InReplyToo = body.MsgID
		body.ID = fmt.Sprintf("%s-%d", msg.Source, body.MsgID)

		return node.n.Reply(msg.Source, body)
	})

	log.Println("Running unique...")
	if err := node.n.Run(); err != nil {
		panic(err)
	}

}

type UniqueBody struct {
	pkg.Body
	ID string `json:"id,omitempty"`
}
