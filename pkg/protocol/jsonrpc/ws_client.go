package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

type WSClientOption func(*WSClient)

func WithWSNotificationHandler(handler NotificationHandler) WSClientOption {
	return func(c *WSClient) {
		c.notificationHandler = handler
	}
}

type WSClient struct {
	conn                *WSConn
	notificationHandler NotificationHandler
	nextID              atomic.Int64
	started             atomic.Bool

	pendingMu sync.Mutex
	pending   map[string]chan wsClientResponse

	done      chan struct{}
	closeOnce sync.Once
	errMu     sync.RWMutex
	err       error
}

func NewWSClient(conn *WSConn, opts ...WSClientOption) *WSClient {
	client := &WSClient{
		conn:    conn,
		pending: map[string]chan wsClientResponse{},
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *WSClient) Start(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return errors.New("jsonrpc ws client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !c.started.CompareAndSwap(false, true) {
		return nil
	}
	go func() {
		select {
		case <-ctx.Done():
			c.closeWithError(ctx.Err())
		case <-c.done:
		}
	}()
	go c.readLoop(ctx)
	return nil
}

func (c *WSClient) Call(ctx context.Context, method string, params any, out any) error {
	if c == nil {
		return errors.New("jsonrpc ws client is nil")
	}
	id := NumberID(c.nextID.Add(1))
	req, err := NewRequest(id, method, params)
	if err != nil {
		return err
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := resp.Validate(); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("jsonrpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func (c *WSClient) Do(ctx context.Context, req Request) (Response, error) {
	if c == nil || c.conn == nil {
		return Response{}, errors.New("jsonrpc ws client is nil")
	}
	if !c.started.Load() {
		return Response{}, errors.New("jsonrpc ws client is not started")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	key := requestIDKey(req.ID)
	ch := make(chan wsClientResponse, 1)
	if err := c.addPending(key, ch); err != nil {
		return Response{}, err
	}
	if err := c.conn.WriteMessage(ctx, req); err != nil {
		c.removePending(key)
		return Response{}, err
	}
	select {
	case <-ctx.Done():
		c.removePending(key)
		return Response{}, ctx.Err()
	case <-c.done:
		c.removePending(key)
		if err := c.closeErr(); err != nil {
			return Response{}, err
		}
		return Response{}, io.ErrClosedPipe
	case result := <-ch:
		if result.err != nil {
			return Response{}, result.err
		}
		return result.response, nil
	}
}

func (c *WSClient) Close() error {
	return c.closeWithError(io.ErrClosedPipe)
}

func (c *WSClient) readLoop(ctx context.Context) {
	for {
		message, err := c.conn.ReadMessage(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				err = ctxErr
			}
			c.closeWithError(err)
			return
		}
		switch message.Kind {
		case KindResponse:
			if message.Response != nil {
				c.dispatchResponse(*message.Response)
			}
		case KindNotification:
			if message.Notification != nil && c.notificationHandler != nil {
				_ = c.notificationHandler(ctx, *message.Notification)
			}
		}
	}
}

func (c *WSClient) addPending(key string, ch chan wsClientResponse) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("jsonrpc request id is required")
	}
	select {
	case <-c.done:
		if err := c.closeErr(); err != nil {
			return err
		}
		return io.ErrClosedPipe
	default:
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	select {
	case <-c.done:
		if err := c.closeErr(); err != nil {
			return err
		}
		return io.ErrClosedPipe
	default:
	}
	c.pending[key] = ch
	return nil
}

func (c *WSClient) removePending(key string) {
	c.pendingMu.Lock()
	delete(c.pending, strings.TrimSpace(key))
	c.pendingMu.Unlock()
}

func (c *WSClient) dispatchResponse(response Response) {
	key := requestIDKey(response.ID)
	if key == "" {
		return
	}
	c.pendingMu.Lock()
	ch := c.pending[key]
	if ch != nil {
		delete(c.pending, key)
	}
	c.pendingMu.Unlock()
	if ch != nil {
		ch <- wsClientResponse{response: response}
	}
}

func (c *WSClient) closeWithError(err error) error {
	if c == nil {
		return nil
	}
	if err == nil {
		err = io.EOF
	}
	c.closeOnce.Do(func() {
		c.errMu.Lock()
		c.err = err
		c.errMu.Unlock()

		c.pendingMu.Lock()
		for key, ch := range c.pending {
			delete(c.pending, key)
			ch <- wsClientResponse{err: err}
		}
		c.pendingMu.Unlock()
		close(c.done)
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusNormalClosure, "")
		}
	})
	return c.closeErr()
}

func (c *WSClient) closeErr() error {
	if c == nil {
		return nil
	}
	c.errMu.RLock()
	defer c.errMu.RUnlock()
	return c.err
}

type wsClientResponse struct {
	response Response
	err      error
}

var _ Requester = (*WSClient)(nil)
