package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{clients: map[*wsClient]struct{}{}}
}

func (h *wsHub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *wsHub) remove(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *wsHub) broadcast(payload any) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.sendBytes(buf)
	}
}

func (h *wsHub) broadcastByTag(payload any, tag string) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.isSubscribed(tag) {
			c.sendBytes(buf)
		}
	}
}

type wsClient struct {
	server   *Server
	conn     *websocket.Conn
	send     chan []byte
	canWrite bool

	subMu    sync.RWMutex
	subAll   bool
	subNames map[string]struct{}
}

func (c *wsClient) isSubscribed(tag string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	if c.subAll {
		return true
	}
	_, ok := c.subNames[strings.ToLower(strings.TrimSpace(tag))]
	return ok
}

func (c *wsClient) setSubs(tags []string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subAll = false
	c.subNames = map[string]struct{}{}
	for _, t := range tags {
		key := strings.ToLower(strings.TrimSpace(t))
		if key == "" {
			continue
		}
		if key == "*" {
			c.subAll = true
			c.subNames = map[string]struct{}{}
			return
		}
		c.subNames[key] = struct{}{}
	}
}

func (c *wsClient) sendBytes(buf []byte) {
	select {
	case c.send <- append([]byte(nil), buf...):
	default:
	}
}

func (c *wsClient) sendJSON(payload any) {
	buf, err := json.Marshal(payload)
	if err != nil {
		return
	}
	c.sendBytes(buf)
}

type wsSetRequest struct {
	client *wsClient
	id     string
	name   string
	value  any
	raw    string
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &wsClient{
		server:   s,
		conn:     conn,
		send:     make(chan []byte, 128),
		subNames: map[string]struct{}{},
		canWrite: !s.cfg.ReadOnly && s.hasValidToken(r),
	}
	s.ws.add(client)

	go client.writer()
	go client.reader()
}

func (c *wsClient) writer() {
	defer func() {
		c.server.ws.remove(c)
		_ = c.conn.Close()
	}()
	for msg := range c.send {
		_ = c.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (c *wsClient) reader() {
	defer func() {
		c.server.ws.remove(c)
		close(c.send)
		_ = c.conn.Close()
	}()
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		op, _ := msg["op"].(string)
		switch strings.ToLower(strings.TrimSpace(op)) {
		case "sub":
			var tags []string
			if arr, ok := msg["tags"].([]any); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						tags = append(tags, s)
					}
				}
			}
			c.setSubs(tags)
			filter := map[string]struct{}{}
			if !c.subAll {
				for t := range c.subNames {
					filter[t] = struct{}{}
				}
			} else {
				filter = nil
			}
			c.sendJSON(map[string]any{
				"op":   "snap",
				"ts":   c.server.cfg.Now().UTC().Format(time.RFC3339Nano),
				"tags": c.server.snapshotTags(filter),
			})
		case "set":
			id, _ := msg["id"].(string)
			name, _ := msg["name"].(string)
			raw, _ := msg["raw"].(string)
			req := wsSetRequest{
				client: c,
				id:     id,
				name:   name,
				value:  msg["value"],
				raw:    raw,
			}
			select {
			case c.server.setReqCh <- req:
			default:
				c.sendJSON(map[string]any{"op": "ack", "id": id, "ok": false, "err": "busy"})
			}
		case "ping":
			c.sendJSON(map[string]any{"op": "pong"})
		}
	}
}

func (s *Server) coalesceWSWrites(ctx context.Context) {
	type pendingBucket struct {
		reqs []wsSetRequest
		last wsSetRequest
	}
	pending := map[string]pendingBucket{}
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	flush := func() {
		for key, bucket := range pending {
			delete(pending, key)
			tag, ok := s.findTag(bucket.last.name)
			if !ok {
				for _, req := range bucket.reqs {
					req.client.sendJSON(map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": false, "err": "tag_not_found"})
				}
				continue
			}
			raw, value, code, err := s.applyWrite(context.Background(), tag, bucket.last.value, bucket.last.raw)
			okWrite := err == nil
			for _, req := range bucket.reqs {
				msg := map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": okWrite}
				if okWrite {
					msg["raw"] = spacedHex(raw)
					msg["value"] = value
				} else {
					msg["err"] = code
				}
				req.client.sendJSON(msg)
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-s.setReqCh:
			if s.cfg.ReadOnly {
				req.client.sendJSON(map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": false, "err": "read_only"})
				continue
			}
			if !req.client.canWrite {
				req.client.sendJSON(map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": false, "err": "unauthorized"})
				continue
			}
			tag, ok := s.findTag(req.name)
			if !ok {
				req.client.sendJSON(map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": false, "err": "tag_not_found"})
				continue
			}
			analog := isCoalescedAnalogType(tag.Spec.Name) && strings.TrimSpace(req.raw) == ""
			if !analog {
				raw, value, code, err := s.applyWrite(ctx, tag, req.value, req.raw)
				msg := map[string]any{"op": "ack", "id": req.id, "name": req.name, "ok": err == nil}
				if err != nil {
					msg["err"] = code
				} else {
					msg["raw"] = spacedHex(raw)
					msg["value"] = value
				}
				req.client.sendJSON(msg)
				continue
			}
			key := strings.ToLower(tag.Key)
			b := pending[key]
			b.reqs = append(b.reqs, req)
			b.last = req
			pending[key] = b
		case <-tick.C:
			flush()
		}
	}
}

func isCoalescedAnalogType(typeName string) bool {
	switch strings.ToUpper(strings.TrimSpace(typeName)) {
	case "INT", "UINT", "WORD", "DINT", "UDINT", "DWORD", "REAL", "TIME":
		return true
	default:
		return false
	}
}
