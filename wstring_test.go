package goprotos7

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWStringDataType(t *testing.T) {
	if WSTRING.String() != "WSTRING[n]" || DataTypeDepara["WSTRING"] != WSTRING ||
		DataTypeDepara["WSTRING[n]"] != WSTRING || DataTypeSize[WSTRING] != 4 ||
		OrderedDataTypes[len(OrderedDataTypes)-1] != WSTRING {
		t.Fatal("inconsistent WSTRING metadata")
	}
	if STRING != 16 || DataTypeSize[STRING] != 2 || STRING.String() != "STRING[n]" {
		t.Fatal("STRING metadata changed")
	}
}

func TestAGWriteDBWString(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		value    string
		want     []byte
	}{
		{"empty", 2, "", []byte{0, 2, 0, 0, 0, 0, 0, 0}},
		{"minimum capacity", 1, "x", []byte{0, 1, 0, 1, 0, 'x'}},
		{"ascii", 3, "AB", []byte{0, 3, 0, 2, 0, 'A', 0, 'B', 0, 0}},
		{"full BMP", 3, "A\u00e9\u4e2d", []byte{0, 3, 0, 3, 0, 'A', 0, 0xe9, 0x4e, 0x2d}},
		{"embedded NUL", 2, "\x00A", []byte{0, 2, 0, 2, 0, 0, 0, 'A'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "DB1.bin")
			initial := bytes.Repeat([]byte{0xaa}, len(tt.want)+4)
			if err := os.WriteFile(path, initial, 0o600); err != nil {
				t.Fatal(err)
			}
			c := &Connection{options: &Options{BinFilesFolder: dir}}
			if err := c.AGWriteDBWString(1, 2, tt.capacity, tt.value); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]byte{0xaa, 0xaa}, tt.want...)
			want = append(want, 0xaa, 0xaa)
			if !bytes.Equal(got, want) {
				t.Fatalf("got % x, want % x", got, want)
			}
		})
	}
}

func TestAGWriteDBWStringMaxCapacity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DB1.bin")
	if err := os.WriteFile(path, make([]byte, 32768), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Connection{options: &Options{BinFilesFolder: dir}}
	if err := c.AGWriteDBWString(1, 0, 16382, strings.Repeat("x", 16382)); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:6], []byte{0x3f, 0xfe, 0x3f, 0xfe, 0, 'x'}) ||
		!bytes.Equal(got[len(got)-2:], []byte{0, 'x'}) {
		t.Fatalf("unexpected maximum-length WSTRING header or content: % x ... % x", got[:6], got[len(got)-2:])
	}
}

func TestAGWriteDBWStringRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DB1.bin")
	initial := bytes.Repeat([]byte{0xaa}, 16)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Connection{options: &Options{BinFilesFolder: dir}}
	tests := []struct {
		name     string
		db       int
		start    int
		capacity int
		value    string
		bounds   bool
	}{
		{"negative DB", -1, 0, 1, "x", false},
		{"DB too large", 65536, 0, 1, "x", false},
		{"negative offset", 1, -1, 1, "x", false},
		{"zero capacity", 1, 0, 0, "", false},
		{"negative capacity", 1, 0, -1, "", false},
		{"capacity too large", 1, 0, 16383, "", false},
		{"value too long", 1, 0, 1, "ab", false},
		{"invalid UTF-8", 1, 0, 2, "\xff", false},
		{"non-BMP character", 1, 0, 2, "\U0001f600", false},
		{"field out of bounds", 1, 12, 1, "a", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.AGWriteDBWString(tt.db, tt.start, tt.capacity, tt.value)
			if err == nil || (tt.bounds && !errors.Is(err, ErrorOutOfBounds)) {
				t.Fatalf("unexpected error: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, initial) {
				t.Fatalf("invalid write changed DB: % x", got)
			}
		})
	}
	if strconv.IntSize == 64 {
		start := int64(1<<32 - 4)
		if err := c.AGWriteDBWString(1, int(start), 1, "x"); err == nil {
			t.Fatal("expected address overflow to fail")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, initial) {
			t.Fatalf("overflowing write changed DB: % x", got)
		}
	}
}

func TestWriteVarWStringBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DB200.bin")
	initial := bytes.Repeat([]byte{0xaa}, 18)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	packet := []byte{
		0x03, 0x00, 0x00, 0x31, 0x02, 0xf0, 0x80, 0x32, 0x01, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x0e, 0x00, 0x12, 0x05, 0x01, 0x12, 0x0a, 0x10, 0x02, 0x00,
		0x0e, 0x00, 0xc8, 0x84, 0x00, 0x00, 0x10, 0x00, 0x04, 0x00, 0x70,
		0x00, 0x05, 0x00, 0x03, 0x00, 0x41, 0x00, 0xe9, 0x4e, 0x2d, 0x00, 0x00, 0x00, 0x00,
	}
	msg, err := unpackWriteVarPacket(packet)
	if err != nil {
		t.Fatal(err)
	}
	c := &Connection{options: &Options{BinFilesFolder: dir}}
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.eventS7FuncWriteVar(msg, server)
	}()
	if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	ack := make([]byte, 256)
	n, err := client.Read(ack)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if n < 3 || !bytes.Equal(ack[n-3:n], []byte{0x05, 0x01, 0xff}) {
		t.Fatalf("unexpected WriteVar ACK: % x", ack[:n])
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0xaa, 0xaa}, packet[len(packet)-14:]...)
	want = append(want, 0xaa, 0xaa)
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestAGWriteDBPreservesStringBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DB1.bin")
	if err := os.WriteFile(path, make([]byte, 8), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Connection{options: &Options{BinFilesFolder: dir}}
	value := []byte{4, 2, 'O', 'K', 0, 0}
	if err := c.AGWriteDB(1, 1, len(value), value); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte{0, 4, 2, 'O', 'K', 0, 0, 0}) {
		t.Fatalf("unexpected STRING bytes: % x", got)
	}
}
