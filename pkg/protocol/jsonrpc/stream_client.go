package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

type NotificationHandler func(context.Context, Notification) error

type StreamClientOption func(*StreamClient)

func WithNotificationHandler(handler NotificationHandler) StreamClientOption {
	return func(c *StreamClient) {
		c.notificationHandler = handler
	}
}

type StreamClient struct {
	stream              *Stream
	notificationHandler NotificationHandler
	nextID              atomic.Int64
	started             atomic.Bool

	pendingMu sync.Mutex
	pending   map[string]chan streamClientResponse

	done      chan struct{}
	closeOnce sync.Once
	errMu     sync.RWMutex
	err       error
}

func NewStreamClient(stream *Stream, opts ...StreamClientOption) *StreamClient {
	client := &StreamClient{
		stream:  stream,
		pending: map[string]chan streamClientResponse{},
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	return client
}

func (c *StreamClient) Start(ctx context.Context) error {
	if c == nil || c.stream == nil {
		return errors.New("jsonrpc stream client is nil")
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

func (c *StreamClient) Call(ctx context.Context, method string, params any, out any) error {
	if c == nil {
		return errors.New("jsonrpc stream client is nil")
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
		return NewRPCError(resp.Error)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func (c *StreamClient) Do(ctx context.Context, req Request) (Response, error) {
	if c == nil || c.stream == nil {
		return Response{}, errors.New("jsonrpc stream client is nil")
	}
	if !c.started.Load() {
		return Response{}, errors.New("jsonrpc stream client is not started")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	key := requestIDKey(req.ID)
	ch := make(chan streamClientResponse, 1)
	if err := c.addPending(key, ch); err != nil {
		return Response{}, err
	}
	if err := c.stream.WriteMessage(req); err != nil {
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

func (c *StreamClient) Close() error {
	return c.closeWithError(io.ErrClosedPipe)
}

func (c *StreamClient) readLoop(ctx context.Context) {
	for {
		message, err := c.stream.ReadMessage()
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

func (c *StreamClient) addPending(key string, ch chan streamClientResponse) error {
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

func (c *StreamClient) removePending(key string) {
	c.pendingMu.Lock()
	delete(c.pending, strings.TrimSpace(key))
	c.pendingMu.Unlock()
}

func (c *StreamClient) dispatchResponse(response Response) {
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
		ch <- streamClientResponse{response: response}
	}
}

func (c *StreamClient) closeWithError(err error) error {
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
			ch <- streamClientResponse{err: err}
		}
		c.pendingMu.Unlock()
		close(c.done)
		if c.stream != nil {
			_ = c.stream.Close()
		}
	})
	return c.closeErr()
}

func (c *StreamClient) closeErr() error {
	if c == nil {
		return nil
	}
	c.errMu.RLock()
	defer c.errMu.RUnlock()
	return c.err
}

func requestIDKey(id RequestID) string {
	return strings.TrimSpace(id.String())
}

type streamClientResponse struct {
	response Response
	err      error
}

var _ Requester = (*StreamClient)(nil)
