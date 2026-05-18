package protocol

import (
	"context"
	"errors"
	"net"

	fb "github.com/dlahoza/pluginart/pkg/protocol/fb/pluginart"
)

func (s *Server) serve() error {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go handleConn(c, s.handler, s.contractHash)
	}
}

func handleConn(c net.Conn, handler Handler, contractHash string) {
	conn := newConn(c)
	defer func() { _ = conn.Close() }()

	msgType, payload, err := conn.Recv()
	if err != nil || msgType != MsgHandshakeRequest {
		return
	}

	req := fb.GetRootAsHandshakeRequest(payload, 0)
	if string(req.ContractHash()) != contractHash {
		_ = conn.Send(MsgHandshakeResponse, buildHandshakeResponse(false, "contract hash mismatch"))
		return
	}
	if err := conn.Send(MsgHandshakeResponse, buildHandshakeResponse(true, "")); err != nil {
		return
	}

	ctx := context.Background()
	for {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return
		}
		switch msgType {
		case MsgCallRequest:
			result, err := handler.Handle(ctx, payload)
			if err != nil {
				_ = conn.Send(MsgPluginError, buildPluginError(1, err.Error(), false))
			} else {
				_ = conn.Send(MsgCallResponse, result)
			}
		case MsgPing:
			p := fb.GetRootAsPing(payload, 0)
			_ = conn.Send(MsgPong, buildPong(p.Seq()))
		case MsgCancel:
			// no-op for v0.1
		default:
			return
		}
	}
}
