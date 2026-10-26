package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// cdpClient is a minimal Chrome DevTools Protocol client: one WebSocket, one id
// counter, one pending map. Events (messages without an id) are ignored except
// for the ones a driver session subscribes to.
type cdpClient struct {
	conn *wsConn
	url  string

	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpReply
	events  func(method string, params json.RawMessage)
	closed  bool
	readErr error
	done    chan struct{}
}

type cdpReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type cdpEnvelope struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// dialCDP connects to a target's DevTools WebSocket endpoint.
func dialCDP(wsURL string, timeout time.Duration) (*cdpClient, error) {
	conn, err := dialWS(wsURL, timeout)
	if err != nil {
		return nil, err
	}
	client := &cdpClient{
		conn:    conn,
		url:     wsURL,
		pending: map[int]chan cdpReply{},
		done:    make(chan struct{}),
	}
	go client.readLoop()
	return client, nil
}

func (c *cdpClient) readLoop() {
	defer close(c.done)
	for {
		raw, err := c.conn.readMessage()
		if err != nil {
			c.mu.Lock()
			c.readErr = err
			closed := c.closed
			pending := c.pending
			c.pending = map[int]chan cdpReply{}
			c.mu.Unlock()
			if !closed {
				for _, ch := range pending {
					select {
					case ch <- cdpReply{Error: &struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}{Code: -1, Message: "connection closed"}}:
					default:
					}
				}
			}
			return
		}

		var envelope cdpEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}

		if envelope.ID != 0 {
			c.mu.Lock()
			ch, ok := c.pending[envelope.ID]
			delete(c.pending, envelope.ID)
			c.mu.Unlock()
			if ok {
				ch <- cdpReply{Result: envelope.Result, Error: envelope.Error}
			}
			continue
		}

		if envelope.Method != "" && c.events != nil {
			c.events(envelope.Method, envelope.Params)
		}
	}
}

// call sends a protocol command and waits for its reply.
func (c *cdpClient) call(method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("cdp connection closed")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan cdpReply, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	payload, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, fmt.Errorf("encode cdp call: %w", err)
	}
	if err := c.conn.writeText(payload); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	select {
	case reply := <-ch:
		if reply.Error != nil {
			return nil, fmt.Errorf("%s: %s (code %d)", method, reply.Error.Message, reply.Error.Code)
		}
		return reply.Result, nil
	case <-time.After(timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("%s: timed out after %s", method, timeout)
	}
}

// evaluate runs JS in the page and returns the runtime result envelope.
func (c *cdpClient) evaluate(expression string, timeout time.Duration) (json.RawMessage, error) {
	return c.call("Runtime.evaluate", map[string]any{
		"expression":                  expression,
		"returnByValue":               true,
		"awaitPromise":                true,
		"userGesture":                 true,
		"allowUnsafeEvalBlockedByCSP": true,
	}, timeout)
}

// evalValue runs JS and decodes the returned value into target.
func (c *cdpClient) evalValue(expression string, timeout time.Duration, target any) error {
	raw, err := c.evaluate(expression, timeout)
	if err != nil {
		return err
	}
	var envelope struct {
		Result struct {
			Type        string          `json:"type"`
			Value       json.RawMessage `json:"value"`
			Description string          `json:"description"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception *struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode evaluate result: %w", err)
	}
	if envelope.ExceptionDetails != nil {
		detail := envelope.ExceptionDetails.Text
		if envelope.ExceptionDetails.Exception != nil && envelope.ExceptionDetails.Exception.Description != "" {
			detail = envelope.ExceptionDetails.Exception.Description
		}
		return fmt.Errorf("page script error: %s", strings.TrimSpace(detail))
	}
	if target == nil {
		return nil
	}
	if len(envelope.Result.Value) == 0 || string(envelope.Result.Value) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Result.Value, target)
}

// evalString is the common case: run JS, get a string back.
func (c *cdpClient) evalString(expression string, timeout time.Duration) (string, error) {
	var out string
	if err := c.evalValue(expression, timeout, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (c *cdpClient) close() error {
	c.mu.Lock()
	already := c.closed
	c.closed = true
	c.mu.Unlock()
	if already {
		return nil
	}
	return c.conn.close()
}
