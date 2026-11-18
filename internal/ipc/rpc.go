package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

const (
	JSONRPCVersion = "2.0"

	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603

	MethodEventsSubscribe = "events.subscribe"
)

// RPCError models a standard JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error (%d): %s", e.Code, e.Message)
}

// EventNotification represents an asynchronous server-pushed event.
type EventNotification struct {
	Topic  string          `json:"topic"`
	Params json.RawMessage `json:"params"`
}

type rpcEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Handler handles one JSON-RPC 2.0 request.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Server serves JSON-RPC 2.0 requests and broadcasts events to subscribed clients.
type Server struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	clients  map[*serverConn]struct{}
}

type serverConn struct {
	conn       net.Conn
	writeMu    sync.Mutex
	subscribed atomic.Bool
}

// NewServer constructs a JSON-RPC 2.0 server with built-in event subscription support.
func NewServer() *Server {
	s := &Server{
		handlers: make(map[string]Handler),
		clients:  make(map[*serverConn]struct{}),
	}
	return s
}

// Register binds method to handler.
func (s *Server) Register(method string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = handler
}

// PublishEvent broadcasts an asynchronous notification frame to all subscribed clients.
func (s *Server) PublishEvent(topic string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event %q: %w", topic, err)
	}
	frame := rpcEnvelope{
		JSONRPC: JSONRPCVersion,
		Method:  "event." + topic,
		Params:  raw,
	}

	s.mu.RLock()
	targets := make([]*serverConn, 0, len(s.clients))
	for sc := range s.clients {
		if sc.subscribed.Load() {
			targets = append(targets, sc)
		}
	}
	s.mu.RUnlock()

	for _, sc := range targets {
		_ = sc.send(frame)
	}
	return nil
}

// Serve accepts connections on ln until ctx is cancelled or ln is closed.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		s.closeAllClients()
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		sc := &serverConn{conn: conn}
		s.addClient(sc)
		wg.Add(1)
		go func(c *serverConn) {
			defer wg.Done()
			defer s.removeClient(c)
			defer c.conn.Close()
			s.serveConn(ctx, c)
		}(sc)
	}
}

func (s *Server) addClient(sc *serverConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[sc] = struct{}{}
}

func (s *Server) removeClient(sc *serverConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, sc)
}

func (s *Server) closeAllClients() {
	s.mu.RLock()
	conns := make([]net.Conn, 0, len(s.clients))
	for sc := range s.clients {
		conns = append(conns, sc.conn)
	}
	s.mu.RUnlock()

	for _, c := range conns {
		_ = c.Close()
	}
}

func (sc *serverConn) send(env rpcEnvelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	sc.writeMu.Lock()
	defer sc.writeMu.Unlock()
	_, err = sc.conn.Write(data)
	return err
}

func (s *Server) serveConn(ctx context.Context, sc *serverConn) {
	reader := bufio.NewReader(sc.conn)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			s.handleFrame(ctx, sc, line)
		}
		if err != nil {
			return
		}
	}
}

func (s *Server) handleFrame(ctx context.Context, sc *serverConn, line []byte) {
	var req rpcEnvelope
	if err := json.Unmarshal(line, &req); err != nil {
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      json.RawMessage("null"),
			Error:   &RPCError{Code: ErrCodeParseError, Message: "parse error"},
		})
		return
	}
	if req.JSONRPC != JSONRPCVersion || req.Method == "" {
		id := req.ID
		if len(id) == 0 {
			id = json.RawMessage("null")
		}
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      id,
			Error:   &RPCError{Code: ErrCodeInvalidRequest, Message: "invalid json-rpc 2.0 request"},
		})
		return
	}

	if req.Method == MethodEventsSubscribe {
		sc.subscribed.Store(true)
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  json.RawMessage(`{"subscribed":true}`),
		})
		return
	}

	s.mu.RLock()
	handler, ok := s.handlers[req.Method]
	s.mu.RUnlock()
	if !ok {
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("method %q not found", req.Method)},
		})
		return
	}

	res, err := handler(ctx, req.Params)
	if err != nil {
		var rpcErr *RPCError
		if !errors.As(err, &rpcErr) {
			rpcErr = &RPCError{Code: ErrCodeInternalError, Message: err.Error()}
		}
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   rpcErr,
		})
		return
	}

	rawRes, marshalErr := json.Marshal(res)
	if marshalErr != nil {
		_ = sc.send(rpcEnvelope{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: ErrCodeInternalError, Message: marshalErr.Error()},
		})
		return
	}
	_ = sc.send(rpcEnvelope{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  rawRes,
	})
}

// Client connects to a Strikeye JSON-RPC 2.0 IPC server and receives asynchronous events.
type Client struct {
	conn    net.Conn
	writeMu sync.Mutex
	nextID  atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan rpcEnvelope

	events chan EventNotification
	closed chan struct{}
	once   sync.Once
}

// NewClient wraps an established net.Conn and starts its background frame reader.
func NewClient(conn net.Conn) *Client {
	c := &Client{
		conn:    conn,
		pending: make(map[int64]chan rpcEnvelope),
		events:  make(chan EventNotification, 64),
		closed:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// DialClient dials addr and returns a ready Client.
func DialClient(ctx context.Context, addr string) (*Client, error) {
	conn, err := Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

// Events returns the channel receiving server-pushed notifications in arrival order.
func (c *Client) Events() <-chan EventNotification {
	return c.events
}

// Subscribe enables event notifications on this connection.
func (c *Client) Subscribe(ctx context.Context) error {
	return c.Call(ctx, MethodEventsSubscribe, nil, nil)
}

// Call invokes method with params and unmarshals result into out (if non-nil).
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	id := c.nextID.Add(1)
	rawID, _ := json.Marshal(id)

	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		rawParams = b
	}

	ch := make(chan rpcEnvelope, 1)
	c.pendingMu.Lock()
	select {
	case <-c.closed:
		c.pendingMu.Unlock()
		return net.ErrClosed
	default:
		c.pending[id] = ch
	}
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	reqBytes, err := json.Marshal(rpcEnvelope{
		JSONRPC: JSONRPCVersion,
		ID:      rawID,
		Method:  method,
		Params:  rawParams,
	})
	if err != nil {
		return err
	}
	reqBytes = append(reqBytes, '\n')

	c.writeMu.Lock()
	_, writeErr := c.conn.Write(reqBytes)
	c.writeMu.Unlock()
	if writeErr != nil {
		return writeErr
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return io.EOF
		}
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return io.EOF
	}
}

// Close closes the underlying transport connection and terminates the reader loop.
func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		err = c.conn.Close()
	})
	return err
}

func (c *Client) readLoop() {
	defer func() {
		_ = c.Close()
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
		close(c.events)
	}()

	reader := bufio.NewReader(c.conn)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var env rpcEnvelope
			if jsonErr := json.Unmarshal(line, &env); jsonErr == nil {
				if len(env.ID) > 0 && string(env.ID) != "null" {
					var id int64
					if idErr := json.Unmarshal(env.ID, &id); idErr == nil {
						c.pendingMu.Lock()
						ch, ok := c.pending[id]
						c.pendingMu.Unlock()
						if ok {
							ch <- env
						}
					}
				} else if len(env.Method) > 6 && env.Method[:6] == "event." {
					select {
					case c.events <- EventNotification{Topic: env.Method[6:], Params: env.Params}:
					case <-c.closed:
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}
