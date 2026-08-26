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

func (c *Client) Read(address string, size int) (any, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}
	buf := make([]byte, size)
	v, err := c.client.Read(address, buf)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) ReadBool(ctx context.Context, db, by, bit int) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	if bit < 0 || bit > 7 {
		return false, fmt.Errorf("invalid bit offset %d", bit)
	}
	buf, err := c.ReadDB(db, by, 1)
	if err != nil {
		return false, err
	}
	return buf[0]&(1<<bit) != 0, nil
}

func (c *Client) WriteBool(ctx context.Context, db, by, bit int, value bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if bit < 0 || bit > 7 {
		return fmt.Errorf("invalid bit offset %d", bit)
	}
	current := []byte{0}
	if c.client == nil {
		return fmt.Errorf("not connected")
	}
	if err := c.client.AGReadDB(db, by, 1, current); err != nil {
		return err
	}
	mask := byte(1 << bit)
	if value {
		current[0] |= mask
	} else {
		current[0] &^= mask
	}
	return c.client.AGWriteDB(db, by, 1, current)
}
