package ssh

import (
	"sync"

	"ctrwatch/src/config"
	"ctrwatch/src/runtime"
	"ctrwatch/src/utils"
)

type ServerSession struct {
	server config.Server

	// TODO(session): promote reconnect state here so the UI can render
	// stale/failed server status without reaching into tunnel internals.
	mu      sync.Mutex
	tunnel  *serverTunnel
	client  *runtime.Client
	state   string
	lastErr error
}

func NewServerSession(server config.Server) *ServerSession {
	return &ServerSession{server: server}
}

func (s *ServerSession) Server() config.Server {
	return s.server
}

func (s *ServerSession) Connect() (*runtime.Client, error) {
	utils.Debugf("ssh session connect start host=%q socket=%q", s.server.Host, s.server.Socket)
	s.mu.Lock()
	if s.client != nil && s.state == "connected" {
		client := s.client
		s.mu.Unlock()
		utils.Debugf("ssh session reuse connected host=%q socket=%q", s.server.Host, s.server.Socket)
		return client, nil
	}
	// TODO(session): if the tunnel already exists, reuse its live socket and
	// only rebuild the runtime client when the transport actually changes.
	s.state = "connecting"
	s.lastErr = nil
	s.mu.Unlock()

	client := runtime.NewClientForSocket(s.server.Socket)

	if !config.IsLocalHost(s.server.Host) {
		tunnel := newServerTunnel(s.server.Host, s.server.Socket)
		if err := tunnel.Start(); err != nil {
			s.mu.Lock()
			s.state = "failed"
			s.lastErr = err
			s.mu.Unlock()
			utils.Debugf("ssh session connect failed host=%q socket=%q err=%v", s.server.Host, s.server.Socket, err)
			return nil, err
		}
		s.mu.Lock()
		s.tunnel = tunnel
		s.mu.Unlock()
		client = runtime.NewClientForSocket(tunnel.Socket())
	}
	client.Runtime = runtime.RuntimeKind(s.server.Socket)

	s.mu.Lock()
	s.client = client
	// TODO(session): keep the last failure reason around separately from a
	// successful connect so reconnecting/failed can be shown in the servers view.
	if s.state != "failed" {
		s.state = "connected"
	}
	s.lastErr = nil
	s.mu.Unlock()

	utils.Debugf("ssh session connected host=%q socket=%q runtime=%q clientSocket=%q", s.server.Host, s.server.Socket, client.Runtime, client.SocketPath)
	return client, nil
}

func (s *ServerSession) Disconnect() error {
	s.mu.Lock()
	tunnel := s.tunnel
	client := s.client
	s.tunnel = nil
	s.client = nil
	s.state = "closed"
	s.lastErr = nil
	s.mu.Unlock()

	if tunnel != nil {
		if err := tunnel.Stop(); err != nil {
			utils.Debugf("ssh session disconnect failed host=%q socket=%q err=%v", s.server.Host, s.server.Socket, err)
			return err
		}
	}
	utils.Debugf("ssh session disconnected host=%q socket=%q", s.server.Host, s.server.Socket)
	_ = client
	return nil
}

func (s *ServerSession) State() string {
	s.mu.Lock()
	tunnel := s.tunnel
	state := s.state
	s.mu.Unlock()
	if tunnel != nil {
		return tunnel.State()
	}
	if state == "" {
		return "unknown"
	}
	return state
}

func (s *ServerSession) LastError() error {
	s.mu.Lock()
	tunnel := s.tunnel
	err := s.lastErr
	s.mu.Unlock()
	if tunnel != nil {
		return tunnel.LastError()
	}
	return err
}

func (s *ServerSession) Socket() string {
	s.mu.Lock()
	tunnel := s.tunnel
	s.mu.Unlock()
	if tunnel != nil {
		return tunnel.Socket()
	}
	return s.server.Socket
}

func (s *ServerSession) Client() *runtime.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}
