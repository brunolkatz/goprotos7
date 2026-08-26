package plc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/get-notify/gos7"
)

type Client struct {
	address string
	rack    int
	slot    int
	port    int
	timeout time.Duration

	handler *gos7.TCPClientHandler
	client  gos7.Client
}

func New(address string, rack, slot, port int, timeout time.Duration) *Client {
	return &Client{
		address: address,
		rack:    rack,
		slot:    slot,
		port:    port,
		timeout: timeout,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	if c.handler != nil {
		return nil
	}
	host := net.JoinHostPort(c.address, fmt.Sprintf("%d", c.port))
	h := gos7.NewTCPClientHandler(host, c.rack, c.slot)
	h.Timeout = c.timeout
	h.IdleTimeout = c.timeout
	done := make(chan error, 1)
	go func() {
		done <- h.Connect()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
	}
	c.handler = h
	c.client = gos7.NewClient(h)
	return nil
}

func (c *Client) Close() error {
	if c.handler == nil {
		return nil
	}
	err := c.handler.Close()
	c.handler = nil
	c.client = nil
	return err
}

func (c *Client) ReadDB(db, start, size int) ([]byte, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	buf := make([]byte, size)
	if err := c.client.AGReadDB(db, start, size, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
