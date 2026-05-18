// Package protocol implements the pluginart wire protocol:
// framing, handshake, call dispatch, and Ping/Pong health checks.
//
// Frame layout (little-endian):
//
//	[magic: 4B 0x50 0x4C 0x47 0x4E]["PLGN"][length: uint32][flags: uint8][payload: N bytes]
package protocol

import (
	"context"
	"net"
)

// MsgType identifies the type of a framed message.
type MsgType uint8

// Message type constants for the pluginart wire protocol frame header.
const (
	MsgHandshakeRequest  MsgType = 0x01
	MsgHandshakeResponse MsgType = 0x02
	MsgCallRequest       MsgType = 0x03
	MsgCallResponse      MsgType = 0x04
	MsgPluginError       MsgType = 0x05
	MsgCancel            MsgType = 0x06
	MsgPing              MsgType = 0x07
	MsgPong              MsgType = 0x08
)

// Handler is implemented by the generated plugin dispatcher.
// payload is the raw FlatBuffers CallRequest bytes; the returned bytes
// are the raw FlatBuffers CallResponse payload.
type Handler interface {
	Handle(ctx context.Context, payload []byte) ([]byte, error)
}

// Conn is a framed, protocol-aware connection over a raw net.Conn.
type Conn interface {
	Send(msgType MsgType, payload []byte) error
	Recv() (MsgType, []byte, error)
	Close() error
}

// NewConn wraps a net.Conn in the pluginart framing layer.
func NewConn(c net.Conn) Conn {
	return newConn(c)
}

// Server is the plugin-side server. It accepts connections, performs the
// handshake, and dispatches calls to the provided Handler.
type Server struct {
	ln           net.Listener
	handler      Handler
	contractHash string
}

// NewServer creates a Server. contractHash must match the host's generated contract.go.
func NewServer(ln net.Listener, handler Handler, contractHash string) *Server {
	return &Server{ln: ln, handler: handler, contractHash: contractHash}
}

// Serve accepts connections until the listener is closed.
func (s *Server) Serve() error {
	return s.serve()
}

// Client is the host-side protocol client for a single plugin connection.
type Client struct {
	conn         Conn
	contractHash string
	pluginName   string
}

// Connect dials the given net.Conn, performs the handshake, and returns a ready Client.
func Connect(c net.Conn, pluginName, contractHash string) (*Client, error) {
	return connect(c, pluginName, contractHash)
}

// Call sends a CallRequest and waits for a CallResponse or PluginError.
// payload is the raw FlatBuffers bytes built by the generated client.
func (c *Client) Call(ctx context.Context, payload []byte) ([]byte, error) {
	return c.call(ctx, payload)
}

// Ping sends a Ping and waits for a Pong. Used by the runtime health-check loop.
func (c *Client) Ping(ctx context.Context) error {
	return c.ping(ctx)
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}
