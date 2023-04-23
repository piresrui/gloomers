package pkg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type INode interface {
	Run() error
	Handle(string, HandlerFunc)
	Reply(string, any) error
}

type HandlerFunc func(msg Message) error

type node struct {
	nodeID  string
	nodeIDS []string

	handlers map[string]HandlerFunc

	tokens chan []byte

	wg    sync.WaitGroup
	mutex sync.Mutex

	Stdin io.Reader
	Stout io.Writer
}

func NewNode() INode {
	n := node{
		handlers: make(map[string]HandlerFunc),
		Stdin:    os.Stdin,
		Stout:    os.Stdout,

		tokens: make(chan []byte, 100),
	}

	n.Handle("init", n.handleInit)

	return &n
}

func (n *node) init(nodeID string, nodeIDs []string) {
	n.nodeID = nodeID
	n.nodeIDS = nodeIDs
}

func (n *node) handleInit(msg Message) error {
	var b Body
	if err := json.Unmarshal(msg.Body, &b); err != nil {
		return fmt.Errorf("failed to unmarshal init body")
	}

	n.init(b.NodeID, b.NodeIDs)
	b.Type = "init_ok"
	b.InReplyToo = b.MsgID

	return n.Reply(msg.Source, b)
}

func (n *node) Run() error {
	scanner := bufio.NewScanner(n.Stdin)

	// async send stdin tokens to channel, close channel when no more tokens
	go func() {
		for scanner.Scan() {
			n.tokens <- scanner.Bytes()
		}
		close(n.tokens)

		if err := scanner.Err(); err != nil {
			panic(err)
		}
	}()

	// reads from tokens in main thread and send them to be handled
	for {
		token, ok := <-n.tokens
		if !ok {
			break
		}

		var msg Message
		if err := json.Unmarshal(token, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal msg")
		}

		var body Body
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return fmt.Errorf("failed to unmarshal body")
		}

		f, ok := n.handlers[body.Type]
		if !ok {
			panic("no handler for message type")
		}

		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			f(msg)
		}()
	}

	n.wg.Wait()
	return nil
}

func (n *node) Handle(key string, f HandlerFunc) {
	if _, ok := n.handlers[key]; ok {
		panic("handler already registered")
	}
	n.handlers[key] = f
}

func (n *node) Reply(source string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal body")
	}

	buf, err := json.Marshal(Message{
		Source:      n.nodeID,
		Destination: source,
		Body:        b,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal msg")
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if _, err = n.Stout.Write(buf); err != nil {
		return fmt.Errorf("failed to send message")
	}

	_, err = n.Stout.Write([]byte("\n"))
	return err
}

type Message struct {
	Source      string          `json:"src,omitempty"`
	Destination string          `json:"dest,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
}

type Body struct {
	Type       string   `json:"type,omitempty"`
	MsgID      int      `json:"msg_id,omitempty"`
	NodeID     string   `json:"node_id,omitempty"`
	NodeIDs    []string `json:"node_ids,omitempty"`
	InReplyToo int      `json:"in_reply_to,omitempty"`
}
