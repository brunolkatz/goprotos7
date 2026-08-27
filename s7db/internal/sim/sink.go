package sim

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
	"github.com/brunolkatz/goprotos7/s7db/internal/plc"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
)

type plcTag struct {
	Name  string
	Addr  address.Address
	Spec  decode.TypeSpec
	Order binary.ByteOrder
	Role  string
}

type PLCSink struct {
	client  *plc.Client
	image   *Image
	byName  map[string]plcTag
	readAll []string
}

func NewPLCSink(client *plc.Client, sch schema.Schema, prog *simlang.Program, image *Image) (*PLCSink, error) {
	order, err := decode.ByteOrder(sch.Endian)
	if err != nil {
		return nil, err
	}
	db := sch.DB
	byName := map[string]plcTag{}
	for _, t := range sch.Tags {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		a, err := address.Parse(t.Addr, &db)
		if err != nil {
			return nil, err
		}
		spec, err := decode.ResolveSchemaType(t.Type, a)
		if err != nil {
			if strings.EqualFold(strings.TrimSpace(t.Type), "STRING") {
				spec = decode.TypeSpec{Name: "STRING", StringLen: 254, SizeBytes: 256}
			} else {
				return nil, err
			}
		}
		byName[t.Name] = plcTag{
			Name:  t.Name,
			Addr:  a,
			Spec:  spec,
			Order: order,
			Role:  strings.ToLower(strings.TrimSpace(t.Role)),
		}
	}
	names := make([]string, 0, len(prog.UsedTags))
	for name := range prog.UsedTags {
		if _, ok := byName[name]; ok {
			names = append(names, name)
		}
	}
	return &PLCSink{client: client, image: image, byName: byName, readAll: names}, nil
}

func (s *PLCSink) Pull(ctx context.Context, names []string) error {
	if len(names) == 0 {
		names = s.readAll
	}
	for _, name := range names {
		t, ok := s.byName[name]
		if !ok {
			continue
		}
		raw, err := s.client.ReadDB(t.Addr.DB, t.Addr.Byte, t.Spec.SizeBytes)
		if err != nil {
			return err
		}
		v, err := decode.DecodeRaw(t.Spec, t.Order, raw, t.Addr.Bit)
		if err != nil {
			return err
		}
		if t.Spec.Name == "TIME" {
			switch x := v.(type) {
			case uint32:
				v = time.Duration(x) * time.Millisecond
			}
		}
		_ = s.image.Set(name, v)
	}
	return nil
}

func (s *PLCSink) Push(ctx context.Context, changed []Change) error {
	for _, ch := range changed {
		t, ok := s.byName[ch.Name]
		if !ok {
			continue
		}
		if t.Spec.Name == "BOOL" {
			cur, err := s.client.ReadDB(t.Addr.DB, t.Addr.Byte, 1)
			if err != nil {
				return err
			}
			b, ok := ch.Value.(bool)
			if !ok {
				return fmt.Errorf("bool write expects bool value for %s", ch.Name)
			}
			mask := byte(1 << t.Addr.Bit)
			if b {
				cur[0] |= mask
			} else {
				cur[0] &^= mask
			}
			if err := s.client.WriteDB(t.Addr.DB, t.Addr.Byte, cur); err != nil {
				return err
			}
			continue
		}
		value := ch.Value
		if t.Spec.Name == "TIME" {
			if d, ok := value.(time.Duration); ok {
				value = int64(d / time.Millisecond)
			}
		}
		payload, err := decode.EncodeRaw(t.Spec, t.Order, value)
		if err != nil {
			return err
		}
		if t.Spec.Name == "STRING" && t.Spec.StringLen > 0 {
			want := t.Spec.StringLen + 2
			if len(payload) != want {
				full := make([]byte, want)
				copy(full, payload)
				full[0] = byte(t.Spec.StringLen)
				if len(payload) > 1 {
					full[1] = payload[1]
				}
				payload = full
			}
		}
		if err := s.client.WriteDB(t.Addr.DB, t.Addr.Byte, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *PLCSink) Reconnect(ctx context.Context) error {
	_ = s.client.Close()
	return s.client.Connect(ctx)
}

type DryRunSink struct {
	Base *PLCSink
	Out  io.Writer
}

func (d *DryRunSink) Pull(ctx context.Context, names []string) error {
	if d.Base == nil {
		return nil
	}
	return d.Base.Pull(ctx, names)
}

func (d *DryRunSink) Push(_ context.Context, changed []Change) error {
	if d.Out == nil {
		return nil
	}
	for _, c := range changed {
		fmt.Fprintf(d.Out, "dry-run push %s=%v\n", c.Name, c.Value)
	}
	return nil
}
