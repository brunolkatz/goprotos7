package sim

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
	"gopkg.in/yaml.v3"
)

func TestWStringAbsentYAMLInitDefaultsToEmpty(t *testing.T) {
	var s schema.Schema
	if err := yaml.Unmarshal([]byte(`version: 1
db: 1
endian: big
size: auto
tags:
  - name: WideStatus
    addr: DB1.WSTRING0.2
    type: WSTRING
`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Tags[0].Init != nil {
		t.Fatalf("expected absent YAML init to be nil, got %v", s.Tags[0].Init)
	}
	prog, diags := simlang.Compile("wide.sim", "tick 100ms;\n", s)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	img := NewImageFromSchema(s, prog)
	if img.Err() != nil {
		t.Fatal(img.Err())
	}
	if got, _ := img.Get("WideStatus"); got != "" {
		t.Fatalf("absent WSTRING init must be empty, got %q", got)
	}
}

type plcIOCall struct {
	db    int
	start int
	size  int
}

type memoryPLCStub struct {
	data   []byte
	reads  []plcIOCall
	writes []plcIOCall
}

func (c *memoryPLCStub) ReadDB(db, start, size int) ([]byte, error) {
	c.reads = append(c.reads, plcIOCall{db, start, size})
	return append([]byte(nil), c.data[start:start+size]...), nil
}

func (c *memoryPLCStub) WriteDB(db, start int, data []byte) error {
	c.writes = append(c.writes, plcIOCall{db, start, len(data)})
	copy(c.data[start:], data)
	return nil
}

func (*memoryPLCStub) Close() error                  { return nil }
func (*memoryPLCStub) Connect(context.Context) error { return nil }

func TestPLCSinkWStringRoundTrip(t *testing.T) {
	for _, addr := range []string{"DB7.WSTRING12.4", "WSTRING12.4"} {
		t.Run(addr, func(t *testing.T) {
			s := schema.Schema{DB: 7, Endian: "little", Tags: []schema.Tag{
				{Name: "WideStatus", Addr: addr, Type: "WSTRING"},
			}}
			prog, diags := simlang.Compile("wide.sim", "tick 100ms;\nWideStatus := W\"\\u4e2d\";\n", s)
			if len(diags) != 0 {
				t.Fatal(diags)
			}
			img := NewImageFromSchema(s, prog)
			client := &memoryPLCStub{data: bytes.Repeat([]byte{0xaa}, 32)}
			initial := []byte{0, 4, 0, 4, 0, 'A', 0, 0xe9, 0x4e, 0x2d, 0, 'Z'}
			copy(client.data[12:], initial)
			sink, err := NewPLCSink(client, s, prog, img)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if err := sink.Pull(ctx, nil); err != nil {
				t.Fatal(err)
			}
			if got, _ := img.Get("WideStatus"); got != "A\u00e9\u4e2dZ" {
				t.Fatalf("unexpected initial pull: %v", got)
			}
			changes, err := NewRunner(prog, img).Step()
			if err != nil {
				t.Fatal(err)
			}
			if err := sink.Push(ctx, changes); err != nil {
				t.Fatal(err)
			}
			want := []byte{0, 4, 0, 1, 0x4e, 0x2d, 0, 0, 0, 0, 0, 0}
			if !bytes.Equal(client.data[12:24], want) {
				t.Fatalf("push payload %x, want %x", client.data[12:24], want)
			}
			if !bytes.Equal(client.data[:12], bytes.Repeat([]byte{0xaa}, 12)) ||
				!bytes.Equal(client.data[24:], bytes.Repeat([]byte{0xaa}, 8)) {
				t.Fatalf("push modified bytes outside WSTRING: %x", client.data)
			}
			if err := img.Set("WideStatus", ""); err != nil {
				t.Fatal(err)
			}
			if err := sink.Pull(ctx, []string{"WideStatus"}); err != nil {
				t.Fatal(err)
			}
			if got, _ := img.Get("WideStatus"); got != "\u4e2d" {
				t.Fatalf("unexpected round-trip value: %v", got)
			}
			wantCall := plcIOCall{db: 7, start: 12, size: 12}
			if len(client.reads) != 2 || client.reads[0] != wantCall || client.reads[1] != wantCall {
				t.Fatalf("unexpected PLC reads: %+v", client.reads)
			}
			if len(client.writes) != 1 || client.writes[0] != wantCall {
				t.Fatalf("unexpected PLC writes: %+v", client.writes)
			}
		})
	}
}

func TestWStringImageInitErrors(t *testing.T) {
	s := schema.Schema{DB: 1, Tags: []schema.Tag{
		{Name: "WideStatus", Addr: "DB1.WSTRING0.2", Type: "WSTRING"},
	}}
	prog, diags := simlang.Compile("wide.sim", "tick 100ms;\n", s)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	for _, value := range []string{"abc", "\U0001f600", "\xff"} {
		t.Run(value, func(t *testing.T) {
			s.Tags[0].Init = value
			img := NewImageFromSchema(s, prog)
			if img.Err() == nil || !strings.Contains(img.Err().Error(), "WideStatus initial value") {
				t.Fatalf("expected named initialization error, got %v", img.Err())
			}
			if _, err := NewRunner(prog, img).Step(); err == nil {
				t.Fatal("runner ignored invalid initial value")
			}
			if _, err := NewPLCSink(nil, s, prog, img); err == nil {
				t.Fatal("PLC sink ignored invalid initial value")
			}
		})
	}
	s.Tags[0].Init = nil
	img := NewImageFromSchema(s, prog)
	if img.Err() != nil {
		t.Fatal(img.Err())
	}
	if v, _ := img.Get("WideStatus"); v != "" {
		t.Fatalf("nil WSTRING init must be empty, got %v", v)
	}
	s.Tags[0].Init = "\u00e9\u4e2d"
	img = NewImageFromSchema(s, prog)
	if img.Err() != nil {
		t.Fatal(img.Err())
	}
	if v, _ := img.Get("WideStatus"); v != "\u00e9\u4e2d" {
		t.Fatalf("valid WSTRING initial value was changed: %v", v)
	}
}

func TestStringSchemaInitStillTruncates(t *testing.T) {
	s := schema.Schema{DB: 1, Tags: []schema.Tag{
		{Name: "Status", Addr: "DB1.DBB0", Type: "STRING[2]", Init: "abcd"},
	}}
	prog, diags := simlang.Compile("narrow.sim", "tick 100ms;\n", s)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	img := NewImageFromSchema(s, prog)
	if img.Err() != nil {
		t.Fatal(img.Err())
	}
	if v, _ := img.Get("Status"); v != "ab" {
		t.Fatalf("STRING initial value no longer truncates: %v", v)
	}
}

type readPLCStub struct {
	raw   []byte
	reads int
}

func (c *readPLCStub) ReadDB(db, start, size int) ([]byte, error) {
	c.reads++
	return c.raw, nil
}
func (*readPLCStub) WriteDB(int, int, []byte) error { return nil }
func (*readPLCStub) Close() error                   { return nil }
func (*readPLCStub) Connect(context.Context) error  { return nil }

func TestPLCPullWString(t *testing.T) {
	s := schema.Schema{DB: 1, Tags: []schema.Tag{
		{Name: "WideStatus", Addr: "DB1.WSTRING0.2", Type: "WSTRING"},
	}}
	prog, diags := simlang.Compile("wide.sim", "tick 100ms;\n", s)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	for _, tt := range []struct {
		name string
		max  int
		raw  []byte
		want string
	}{
		{"valid", 2, []byte{0, 2, 0, 2, 0, 0xe9, 0x4e, 0x2d}, ""},
		{"image capacity error", 1, []byte{0, 2, 0, 2, 0, 0xe9, 0x4e, 0x2d}, "PLC pull WideStatus: WSTRING length 2 exceeds max 1"},
		{"malformed header", 2, []byte{0, 1, 0, 0, 0, 0, 0, 0}, "does not match capacity"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := NewImageFromSchema(s, prog)
			img.wstringMaxLen["WideStatus"] = tt.max
			client := &readPLCStub{raw: tt.raw}
			sink := &PLCSink{
				client: client, image: img, readAll: []string{"WideStatus"},
				byName: map[string]plcTag{"WideStatus": {
					Addr:  address.Address{DB: 1, Area: "WSTRING", Bit: 2},
					Spec:  decode.TypeSpec{Name: "WSTRING", StringLen: 2, SizeBytes: 8},
					Order: binary.LittleEndian,
				}},
			}
			err := sink.Pull(context.Background(), nil)
			if client.reads != 1 {
				t.Fatalf("expected PLC read, got %d", client.reads)
			}
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected %q, got %v", tt.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v, _ := img.Get("WideStatus"); v != "\u00e9\u4e2d" {
				t.Fatalf("unexpected pulled value: %v", v)
			}
		})
	}
}
