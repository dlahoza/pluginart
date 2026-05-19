package protocol

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	fb "github.com/dlahoza/pluginart/pkg/protocol/fb/pluginart"
)

type handlerFunc func(context.Context, []byte) ([]byte, error)

func (f handlerFunc) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	return f(ctx, payload)
}

func TestConnSendRecv(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	sendErr := make(chan error, 1)
	go func() {
		sendErr <- NewConn(a).Send(MsgCallRequest, []byte("payload"))
	}()

	msgType, payload, err := NewConn(b).Recv()
	if err != nil {
		t.Fatal(err)
	}
	if err := <-sendErr; err != nil {
		t.Fatal(err)
	}
	if msgType != MsgCallRequest {
		t.Fatalf("msgType = %d", msgType)
	}
	if string(payload) != "payload" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestConnSendRejectsOversizedPayload(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	err := NewConn(a).Send(MsgCallRequest, bytes.Repeat([]byte{'x'}, maxFrameSize+1))
	if err == nil || !strings.Contains(err.Error(), "exceeds max frame size") {
		t.Fatalf("err = %v", err)
	}
}

func TestConnRecvRejectsBadFrames(t *testing.T) {
	t.Run("invalid magic", func(t *testing.T) {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()

		go func() {
			_, _ = a.Write([]byte("BAD!\x00\x00\x00\x00\x01"))
		}()
		_, _, err := NewConn(b).Recv()
		if err == nil || !strings.Contains(err.Error(), "invalid magic") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("oversized length", func(t *testing.T) {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()

		go func() {
			var hdr [headerSize]byte
			copy(hdr[:4], magic[:])
			binary.LittleEndian.PutUint32(hdr[4:8], maxFrameSize+1)
			hdr[8] = byte(MsgCallRequest)
			_, _ = a.Write(hdr[:])
		}()
		_, _, err := NewConn(b).Recv()
		if err == nil || !strings.Contains(err.Error(), "exceeds max frame size") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("short payload", func(t *testing.T) {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()

		go func() {
			var hdr [headerSize]byte
			copy(hdr[:4], magic[:])
			binary.LittleEndian.PutUint32(hdr[4:8], 5)
			hdr[8] = byte(MsgCallRequest)
			_, _ = a.Write(hdr[:])
			_ = a.Close()
		}()
		_, _, err := NewConn(b).Recv()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandshakeBuilders(t *testing.T) {
	req := fb.GetRootAsHandshakeRequest(buildHandshakeRequest("hash", "plugin"), 0)
	if string(req.ContractHash()) != "hash" {
		t.Fatalf("contract hash = %q", req.ContractHash())
	}
	if string(req.PluginName()) != "plugin" {
		t.Fatalf("plugin name = %q", req.PluginName())
	}
	if req.ProtocolVersion() != 1 {
		t.Fatalf("protocol version = %d", req.ProtocolVersion())
	}

	ok := fb.GetRootAsHandshakeResponse(buildHandshakeResponse(true, ""), 0)
	if !ok.Ok() || len(ok.Error()) != 0 {
		t.Fatalf("ok response = ok:%v err:%q", ok.Ok(), ok.Error())
	}

	rejected := fb.GetRootAsHandshakeResponse(buildHandshakeResponse(false, "nope"), 0)
	if rejected.Ok() || string(rejected.Error()) != "nope" {
		t.Fatalf("rejected response = ok:%v err:%q", rejected.Ok(), rejected.Error())
	}
}

func TestPingPongAndPluginErrorBuilders(t *testing.T) {
	ping := fb.GetRootAsPing(buildPing(42), 0)
	if ping.Seq() != 42 {
		t.Fatalf("ping seq = %d", ping.Seq())
	}
	pong := fb.GetRootAsPong(buildPong(43), 0)
	if pong.Seq() != 43 {
		t.Fatalf("pong seq = %d", pong.Seq())
	}
	pe := fb.GetRootAsPluginError(buildPluginError(7, "failed", true), 0)
	if pe.Code() != 7 || string(pe.Message()) != "failed" || !pe.Retry() {
		t.Fatalf("plugin error = code:%d message:%q retry:%v", pe.Code(), pe.Message(), pe.Retry())
	}
}

func TestConnectAndCall(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	go handleConn(b, handlerFunc(func(_ context.Context, payload []byte) ([]byte, error) {
		if string(payload) != "request" {
			t.Errorf("payload = %q", payload)
		}
		return []byte("response"), nil
	}), "hash")

	client, err := Connect(a, "plugin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	got, err := client.Call(context.Background(), []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "response" {
		t.Fatalf("response = %q", got)
	}
}

func TestConnectErrors(t *testing.T) {
	t.Run("send error", func(t *testing.T) {
		a, b := net.Pipe()
		_ = b.Close()
		if _, err := Connect(a, "plugin", "hash"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("recv error", func(t *testing.T) {
		a, b := net.Pipe()
		go func() {
			_, _, _ = NewConn(b).Recv()
			_ = b.Close()
		}()
		if _, err := Connect(a, "plugin", "hash"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("wrong response type", func(t *testing.T) {
		a, b := net.Pipe()
		go func() {
			_, _, _ = NewConn(b).Recv()
			_ = NewConn(b).Send(MsgPong, buildPong(1))
		}()
		if _, err := Connect(a, "plugin", "hash"); err == nil || !strings.Contains(err.Error(), "expected handshake response") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		a, b := net.Pipe()
		go handleConn(b, handlerFunc(func(context.Context, []byte) ([]byte, error) {
			return nil, nil
		}), "other")
		if _, err := Connect(a, "plugin", "hash"); err == nil || !strings.Contains(err.Error(), "handshake rejected") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestClientCallErrors(t *testing.T) {
	t.Run("plugin error", func(t *testing.T) {
		a, b := net.Pipe()
		defer a.Close()
		defer b.Close()
		go handleConn(b, handlerFunc(func(context.Context, []byte) ([]byte, error) {
			return nil, errors.New("boom")
		}), "hash")

		client, err := Connect(a, "plugin", "hash")
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Call(context.Background(), []byte("request"))
		if err == nil || !strings.Contains(err.Error(), "plugin error 1: boom") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unexpected message", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()
		defer peer.Close()

		done := make(chan error, 1)
		go func() {
			_, err := client.Call(context.Background(), []byte("request"))
			done <- err
		}()

		msgType, _, err := NewConn(peer).Recv()
		if err != nil {
			t.Fatal(err)
		}
		if msgType != MsgCallRequest {
			t.Fatalf("msgType = %d", msgType)
		}
		if err := NewConn(peer).Send(MsgCancel, nil); err != nil {
			t.Fatal(err)
		}

		err = <-done
		if err == nil || !strings.Contains(err.Error(), "unexpected message type") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("context canceled after send", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()
		defer peer.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() {
			_, err := client.Call(ctx, []byte("request"))
			done <- err
		}()

		if _, _, err := NewConn(peer).Recv(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("skips ping then recv error", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()

		done := make(chan error, 1)
		go func() {
			_, err := client.Call(context.Background(), []byte("request"))
			done <- err
		}()

		if _, _, err := NewConn(peer).Recv(); err != nil {
			t.Fatal(err)
		}
		if err := NewConn(peer).Send(MsgPing, buildPing(1)); err != nil {
			t.Fatal(err)
		}
		_ = peer.Close()
		if err := <-done; err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestClientPing(t *testing.T) {
	t.Run("success skips other messages", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()
		defer peer.Close()

		done := make(chan error, 1)
		go func() {
			done <- client.Ping(context.Background())
		}()

		msgType, payload, err := NewConn(peer).Recv()
		if err != nil {
			t.Fatal(err)
		}
		if msgType != MsgPing {
			t.Fatalf("msgType = %d", msgType)
		}
		seq := fb.GetRootAsPing(payload, 0).Seq()
		if err := NewConn(peer).Send(MsgCallResponse, []byte("ignored")); err != nil {
			t.Fatal(err)
		}
		if err := NewConn(peer).Send(MsgPong, buildPong(seq)); err != nil {
			t.Fatal(err)
		}

		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})

	t.Run("recv error", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()
		_ = peer.Close()
		if err := client.Ping(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("context canceled after send", func(t *testing.T) {
		client, peer := connectedClient(t)
		defer client.Close()
		defer peer.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() {
			done <- client.Ping(ctx)
		}()

		if _, _, err := NewConn(peer).Recv(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestHandleConnIgnoresBadStart(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	done := make(chan struct{})
	go func() {
		handleConn(b, handlerFunc(func(context.Context, []byte) ([]byte, error) {
			t.Error("handler should not be called")
			return nil, nil
		}), "hash")
		close(done)
	}()

	if err := NewConn(a).Send(MsgPing, buildPing(1)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleConn did not return")
	}
}

func TestHandleConnCancelThenDefault(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	done := make(chan struct{})
	go func() {
		handleConn(b, handlerFunc(func(context.Context, []byte) ([]byte, error) {
			return []byte("unused"), nil
		}), "hash")
		close(done)
	}()

	client, err := Connect(a, "plugin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.conn.Send(MsgCancel, nil); err != nil {
		t.Fatal(err)
	}
	if err := client.conn.Send(MsgHandshakeRequest, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleConn did not return")
	}
}

func TestServerServeReturnsOnClosedListener(t *testing.T) {
	server := NewServer(errorListener{err: net.ErrClosed}, handlerFunc(func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	}), "hash")

	done := make(chan error, 1)
	go func() {
		done <- server.Serve()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestServerServeReturnsAcceptError(t *testing.T) {
	want := errors.New("accept failed")
	server := NewServer(errorListener{err: want}, handlerFunc(func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	}), "hash")
	if err := server.Serve(); !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func connectedClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	conn := NewConn(b)
	go func() {
		msgType, payload, err := conn.Recv()
		if err != nil {
			return
		}
		if msgType != MsgHandshakeRequest {
			return
		}
		req := fb.GetRootAsHandshakeRequest(payload, 0)
		if string(req.ContractHash()) != "hash" {
			_ = conn.Send(MsgHandshakeResponse, buildHandshakeResponse(false, "bad hash"))
			return
		}
		_ = conn.Send(MsgHandshakeResponse, buildHandshakeResponse(true, ""))
	}()
	client, err := Connect(a, "plugin", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return client, b
}

type errorListener struct {
	err error
}

func (l errorListener) Accept() (net.Conn, error) {
	return nil, l.err
}

func (l errorListener) Close() error {
	return nil
}

func (l errorListener) Addr() net.Addr {
	return dummyAddr("error")
}

type dummyAddr string

func (a dummyAddr) Network() string {
	return string(a)
}

func (a dummyAddr) String() string {
	return string(a)
}
