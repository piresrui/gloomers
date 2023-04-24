package main

import (
	"encoding/json"
	"fmt"
	"gloomers/pkg"
	"log"
)

type EchoNode struct {
	n pkg.INode
}

func main() {
	node := EchoNode{
		n: pkg.NewNode(),
	}

	node.n.Handle("echo", func(msg pkg.Message) error {
		var body EchoBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal echo body")
		}

		body.Type = "echo_ok"
		body.InReplyToo = body.MsgID

		return node.n.Reply(msg.Source, body)
	})

	log.Println("Running echo...")
	if err := node.n.Run(); err != nil {
		panic(err)
	}

}

type EchoBody struct {
	pkg.Body
	Echo string `json:"echo,omitempty"`
}
