package goprotos7

import (
	"net"
	"testing"
	"time"
)

func TestReadConnHandlesFragmentedFrames(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		frame := []byte{0x03, 0x00, 0x00, 0x07, 0x02, 0xF0, 0x80}
		_, _ = clientConn.Write(frame[:2])
		time.Sleep(5 * time.Millisecond)
		_, _ = clientConn.Write(frame[2:4])
		time.Sleep(5 * time.Millisecond)
		_, _ = clientConn.Write(frame[4:5])
		time.Sleep(5 * time.Millisecond)
		_, _ = clientConn.Write(frame[5:])
	}()

	got, err := readConn(serverConn)
	if err != nil {
		t.Fatalf("unexpected readConn error: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("unexpected frame size: %d", len(got))
	}
	if got[0] != 0x03 || got[5] != 0xF0 {
		t.Fatalf("unexpected frame content: %v", got)
	}
	<-done
}
