package main

import (
	"encoding/json"
	"fmt"
	"gloomers/pkg"
	"log"
	"sync"
	"time"

	"golang.org/x/exp/maps"
)

type BroadcastNode struct {
	n         pkg.INode
	messages  map[int]bool
	neighbors []string
	known     map[string]map[int]bool
	wg        sync.WaitGroup
}

func main() {
	node := BroadcastNode{
		n:        pkg.NewNode(),
		messages: make(map[int]bool),
		known:    make(map[string]map[int]bool),
	}

	node.n.Handle("broadcast", func(msg pkg.Message) error {
		var body BroadcastBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal broadcast body")
		}

		body.Type = "broadcast_ok"
		body.InReplyToo = body.MsgID
		node.messages[body.Message] = true
		body.Message = 0

		return node.n.Reply(msg.Source, body)
	})

	node.n.Handle("read", func(msg pkg.Message) error {
		var body ReadBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal read body")
		}

		body.Type = "read_ok"
		body.InReplyToo = body.MsgID
		body.Messages = maps.Keys(node.messages)

		return node.n.Reply(msg.Source, body)
	})

	node.n.Handle("topology", func(msg pkg.Message) error {
		var body TopologyBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal topology body")
		}

		body.Type = "topology_ok"
		body.InReplyToo = body.MsgID
		node.neighbors = body.Topology[node.n.NodeID()]
		body.Topology = nil

		return node.n.Reply(msg.Source, body)
	})

	node.n.Handle("gossip", func(msg pkg.Message) error {
		var body GossipBody
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal topology body")
		}

		// we got a injected gossip
		// send a gossip to all neighbors
		if msg.Source == node.n.NodeID() {
			for _, n := range node.neighbors {
				var known_to_n []int
				if l, ok := node.known[n]; ok {
					known_to_n = maps.Keys(l)
				}

				body := GossipBody{
					Messages: difference(maps.Keys(node.messages), known_to_n),
				}
				body.Type = "gossip"
				node.n.Reply(n, body)
			}
			return nil
		}

		// we got a gossip, update our messages
		for _, m := range body.Messages {
			// add to our known messages
			node.messages[m] = true

			// add to messages we know the sender node knows
			if known_to_n := node.known[msg.Source]; known_to_n == nil {
				node.known[msg.Source] = make(map[int]bool)
			}
			node.known[msg.Source][m] = true
		}

		return nil
	})

	log.Println("Running broadcast...")
	end := make(chan any)

	go node.schedule(5*time.Second, end)
	if err := node.n.Run(); err != nil {
		panic(err)
	}
	close(end)
	node.wg.Wait()

}

func (n *BroadcastNode) schedule(d time.Duration, end chan any) {
	n.wg.Add(1)
	go func(end chan any) {
		defer n.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ticker.C:
				body := GossipBody{}
				body.Type = "gossip"
				n.n.Trigger(body.Type, body)
			case <-end:
				return
			}
		}
	}(end)

}

func difference[T comparable](slice1 []T, slice2 []T) []T {
	set := make(map[T]bool)

	if len(slice2) == 0 {
		return slice1
	}

	for _, value := range slice1 {
		set[value] = true
	}
	for _, value := range slice2 {
		delete(set, value)
	}

	return maps.Keys(set)
}

type BroadcastBody struct {
	pkg.Body
	Message int `json:"message,omitempty"`
}

type ReadBody struct {
	pkg.Body
	Messages []int `json:"messages"`
}

type TopologyBody struct {
	pkg.Body
	Topology map[string][]string `json:"topology,omitempty"`
}

type GossipBody struct {
	pkg.Body
	Messages []int `json:"messages,omitempty"`
}
