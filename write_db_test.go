package goprotos7

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnpackWriteVarPacketFromPCAP(t *testing.T) {
	// Captured from docs/pcaps/s7comm_varservice_libnodavedemo.pcap (WriteVar request).
	packet := []byte{
		0x03, 0x00, 0x00, 0x27, 0x02, 0xF0, 0x80, 0x32, 0x01, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x0E, 0x00, 0x08, 0x05, 0x01, 0x12, 0x0A, 0x10, 0x02, 0x00,
		0x04, 0x00, 0x00, 0x83, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x20, 0xA9,
		0x10, 0x00, 0x01,
	}

	msg, err := unpackWriteVarPacket(packet)
	if err != nil {
		t.Fatalf("unexpected unpackWriteVarPacket error: %v", err)
	}
	if msg.S7Request == nil || msg.S7Request.FunctionCode != S7FuncWriteVar {
		t.Fatalf("unexpected write request parse result: %#v", msg.S7Request)
	}
	param, ok := msg.S7Request.FuncParam.(*S7ParamWriteVar)
	if !ok {
		t.Fatalf("unexpected write request param type: %T", msg.S7Request.FuncParam)
	}
	if len(param.Items) != 1 || len(param.DataItems) != 1 {
		t.Fatalf("unexpected items/data items count: %d/%d", len(param.Items), len(param.DataItems))
	}
	if param.Items[0].Area != 0x83 {
		t.Fatalf("unexpected area: %x", param.Items[0].Area)
	}
	if !bytes.Equal(param.DataItems[0].Data, []byte{0xA9, 0x10, 0x00, 0x01}) {
		t.Fatalf("unexpected write payload: % x", param.DataItems[0].Data)
	}
}

func TestEventS7FuncWriteVarWritesDBBinFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "DB200.bin")
	if err := os.WriteFile(dbPath, make([]byte, 16), 0o644); err != nil {
		t.Fatalf("error creating test DB file: %v", err)
	}

	// Same captured write call shape, adapted to DB area (0x84) and DB200.
	packet := []byte{
		0x03, 0x00, 0x00, 0x27, 0x02, 0xF0, 0x80, 0x32, 0x01, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x0E, 0x00, 0x08, 0x05, 0x01, 0x12, 0x0A, 0x10, 0x02, 0x00,
		0x04, 0x00, 0xC8, 0x84, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x20, 0xA9,
		0x10, 0x00, 0x01,
	}

	msg, err := unpackWriteVarPacket(packet)
	if err != nil {
		t.Fatalf("unexpected unpackWriteVarPacket error: %v", err)
	}

	c := &Connection{
		options: &Options{
			BinFilesFolder: tmpDir,
		},
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		c.eventS7FuncWriteVar(msg, serverConn)
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	ack := make([]byte, 256)
	n, err := clientConn.Read(ack)
	if err != nil {
		t.Fatalf("unexpected write response read error: %v", err)
	}
	ack = ack[:n]
	<-done

	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("error reading DB file: %v", err)
	}
	if !bytes.Equal(got[:4], []byte{0xA9, 0x10, 0x00, 0x01}) {
		t.Fatalf("unexpected DB bytes: % x", got[:4])
	}

	if len(ack) < 3 || !bytes.Equal(ack[len(ack)-3:], []byte{0x05, 0x01, 0xFF}) {
		t.Fatalf("unexpected WriteVar ACK trailer: % x", ack)
	}
}
