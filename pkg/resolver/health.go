package resolver

import (
	"context"
	"net"
)

type Health struct {
	addr string
	ln   net.Listener
}

func NewHealth(addr string) (*Health, error) {
	h := &Health{
		addr: addr,
	}

	return h, nil
}

func (h *Health) ListenAndServe() error {
	ln, err := net.Listen("tcp", h.addr)
	if err != nil {
		return err
	}

	h.ln = ln

	for {
		cn, err := ln.Accept()
		if err != nil {
			return err
		}

		go h.handleConnection(cn)
	}
}

func (h *Health) Shutdown(ctx context.Context) error {
	return h.ln.Close()
}

func (h *Health) handleConnection(cn net.Conn) error {
	defer cn.Close()

	if _, err := cn.Write([]byte("ok")); err != nil {
		return err
	}

	return nil
}
