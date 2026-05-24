package jsonrpc

import (
	"context"
	"errors"
	"sync"

	"github.com/coder/websocket"
)

type WSConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func NewWSConn(conn *websocket.Conn) *WSConn {
	return &WSConn{conn: conn}
}

func (c *WSConn) ReadMessage(ctx context.Context) (DecodedMessage, error) {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return DecodedMessage{}, err
	}
	return Decode(data)
}

func (c *WSConn) WriteMessage(ctx context.Context, v any) error {
	data, err := Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (c *WSConn) Close(code websocket.StatusCode, reason string) error {
	return c.conn.Close(code, reason)
}

func ServeWS(ctx context.Context, router *Router, conn *WSConn) error {
	if router == nil {
		return errors.New("jsonrpc router is nil")
	}
	if conn == nil {
		return errors.New("jsonrpc websocket conn is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		message, err := conn.ReadMessage(ctx)
		if err != nil {
			return err
		}
		if message.Kind != KindRequest || message.Request == nil {
			continue
		}
		response := router.Handle(ctx, *message.Request)
		if err := conn.WriteMessage(ctx, response); err != nil {
			return err
		}
	}
}

type WSNotifier struct {
	Conn *WSConn
}

func (n WSNotifier) Notify(ctx context.Context, notification Notification) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	if n.Conn == nil {
		return errors.New("jsonrpc websocket conn is nil")
	}
	return n.Conn.WriteMessage(ctx, notification)
}
