package protocol

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	fb "github.com/dlahoza/pluginart/pkg/protocol/fb/pluginart"
)

var globalSeq atomic.Uint64

func connect(c net.Conn, pluginName, contractHash string) (*Client, error) {
	conn := newConn(c)

	if err := conn.Send(MsgHandshakeRequest, buildHandshakeRequest(contractHash, pluginName)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	msgType, payload, err := conn.Recv()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if msgType != MsgHandshakeResponse {
		_ = conn.Close()
		return nil, fmt.Errorf("expected handshake response, got %d", msgType)
	}
	resp := fb.GetRootAsHandshakeResponse(payload, 0)
	if !resp.Ok() {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake rejected: %s", resp.Error())
	}

	return &Client{conn: conn, contractHash: contractHash, pluginName: pluginName}, nil
}

// locked returns the lockedConn underlying cl.conn for serialised round-trips.
func (cl *Client) locked() *lockedConn {
	return cl.conn.(*lockedConn)
}

func (cl *Client) call(ctx context.Context, payload []byte) ([]byte, error) {
	lc := cl.locked()
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if err := lc.Send(MsgCallRequest, payload); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msgType, data, err := lc.Recv()
		if err != nil {
			return nil, err
		}
		switch msgType {
		case MsgCallResponse:
			return data, nil
		case MsgPluginError:
			pe := fb.GetRootAsPluginError(data, 0)
			return nil, fmt.Errorf("plugin error %d: %s", pe.Code(), pe.Message())
		case MsgPing, MsgPong:
			// skip
		default:
			return nil, fmt.Errorf("unexpected message type %d", msgType)
		}
	}
}

func (cl *Client) ping(ctx context.Context) error {
	lc := cl.locked()
	lc.mu.Lock()
	defer lc.mu.Unlock()

	seq := globalSeq.Add(1)
	if err := lc.Send(MsgPing, buildPing(seq)); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgType, data, err := lc.Recv()
		if err != nil {
			return err
		}
		if msgType == MsgPong {
			pong := fb.GetRootAsPong(data, 0)
			if pong.Seq() == seq {
				return nil
			}
		}
	}
}
