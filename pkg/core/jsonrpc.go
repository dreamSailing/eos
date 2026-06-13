//go:build legacy

package core

import (
	"context"
	"io"

	"github.com/coder/websocket"

	coreapijsonrpc "github.com/dreamSailing/eos/pkg/coreapi/jsonrpc"
	protocoljsonrpc "github.com/dreamSailing/eos/pkg/protocol/jsonrpc"
)

func (r *Runtime) JSONRPCRouter(options ...coreapijsonrpc.Options) (*protocoljsonrpc.Router, error) {
	router := protocoljsonrpc.NewRouter()
	opts := coreapijsonrpc.Options{
		ServerName:      "eos-core",
		ProtocolVersion: "v1",
	}
	if len(options) > 0 {
		opts = options[0]
	}
	if err := coreapijsonrpc.Register(router, NewLegacyEngine(r), opts); err != nil {
		return nil, err
	}
	return router, nil
}

func (r *Runtime) JSONRPCClient(options ...coreapijsonrpc.Options) (*protocoljsonrpc.InProcessClient, error) {
	router, err := r.JSONRPCRouter(options...)
	if err != nil {
		return nil, err
	}
	return protocoljsonrpc.NewInProcessClient(protocoljsonrpc.InProcessServer{Router: router}), nil
}

func (r *Runtime) ServeJSONRPCStream(ctx context.Context, reader io.Reader, writer io.Writer, options ...coreapijsonrpc.Options) error {
	stream := protocoljsonrpc.NewStream(reader, writer)
	opts := coreapijsonrpc.Options{
		ServerName:      "eos-core",
		ProtocolVersion: "v1",
		Notifier:        protocoljsonrpc.StreamNotifier{Stream: stream},
	}
	if len(options) > 0 {
		opts = options[0]
		if opts.Notifier == nil {
			opts.Notifier = protocoljsonrpc.StreamNotifier{Stream: stream}
		}
	}
	router, err := r.JSONRPCRouter(opts)
	if err != nil {
		return err
	}
	return protocoljsonrpc.ServeStream(ctx, router, stream)
}

func (r *Runtime) ServeJSONRPCWS(ctx context.Context, conn *websocket.Conn, options ...coreapijsonrpc.Options) error {
	wsConn := protocoljsonrpc.NewWSConn(conn)
	opts := coreapijsonrpc.Options{
		ServerName:      "eos-core",
		ProtocolVersion: "v1",
		Notifier:        protocoljsonrpc.WSNotifier{Conn: wsConn},
	}
	if len(options) > 0 {
		opts = options[0]
		if opts.Notifier == nil {
			opts.Notifier = protocoljsonrpc.WSNotifier{Conn: wsConn}
		}
	}
	router, err := r.JSONRPCRouter(opts)
	if err != nil {
		return err
	}
	return protocoljsonrpc.ServeWS(ctx, router, wsConn)
}
