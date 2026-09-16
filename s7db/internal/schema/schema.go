package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"gopkg.in/yaml.v3"
)

type SizeSpec struct {
	Auto  bool
	Bytes int
}

func (s SizeSpec) MarshalYAML() (any, error) {
	if s.Auto {
		return "auto", nil
	}
	return s.Bytes, nil
}

func (s *SizeSpec) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!str" && strings.EqualFold(strings.TrimSpace(n.Value), "auto") {
			s.Auto = true
			s.Bytes = 0
			return nil
		}
		var i int
		if err := n.Decode(&i); err == nil {
			if i < 0 {
				return fmt.Errorf("size cannot be negative")
			}
			s.Auto = false
			s.Bytes = i
			return nil
		}
	}
	return fmt.Errorf("size must be integer or auto")
}

type Tag struct {
	Addr      string         `yaml:"addr" json:"addr"`
	Name      string         `yaml:"name,omitempty" json:"name,omitempty"`
	Type      string         `yaml:"type" json:"type"`
	Init      any            `yaml:"init,omitempty" json:"init,omitempty"`
	Desc      string         `yaml:"desc,omitempty" json:"desc,omitempty"`
	Role      string         `yaml:"role,omitempty" json:"role,omitempty"`
	Heartbeat *HeartbeatSpec `yaml:"heartbeat,omitempty" json:"heartbeat,omitempty"`
	UI        *TagUI         `yaml:"ui,omitempty" json:"ui,omitempty"`
}

type HeartbeatSpec struct {
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout  string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Polarity string `yaml:"polarity,omitempty" json:"polarity,omitempty"`
}

type TagUI struct {
	Widget   string   `yaml:"widget,omitempty" json:"widget,omitempty"`
	Mode     string   `yaml:"mode,omitempty" json:"mode,omitempty"`
	Group    string   `yaml:"group,omitempty" json:"group,omitempty"`
	Color    string   `yaml:"color,omitempty" json:"color,omitempty"`
	Confirm  bool     `yaml:"confirm,omitempty" json:"confirm,omitempty"`
	Readonly bool     `yaml:"readonly,omitempty" json:"readonly,omitempty"`
	Min      *float64 `yaml:"min,omitempty" json:"min,omitempty"`
	Max      *float64 `yaml:"max,omitempty" json:"max,omitempty"`
	Step     *float64 `yaml:"step,omitempty" json:"step,omitempty"`
	Unit     string   `yaml:"unit,omitempty" json:"unit,omitempty"`
}

type Diagnostic struct {
	Level   string
	Message string
}

type Schema struct {
	Version   int      `yaml:"version" json:"version"`
	DB        int      `yaml:"db" json:"db"`
	Name      string   `yaml:"name,omitempty" json:"name,omitempty"`
	Optimized bool     `yaml:"optimized,omitempty" json:"optimized,omitempty"`
	Endian    string   `yaml:"endian" json:"endian"`
	Size      SizeSpec `yaml:"size" json:"size"`
	Tags      []Tag    `yaml:"tags" json:"tags"`
}

func New(db int) Schema {
	return Schema{
		Version: 1,
		DB:      db,
		Endian:  "big",
		Size:    SizeSpec{Auto: true},
		Tags:    []Tag{},
	}
}

func Load(path string, defaultDB *int) (Schema, error) {
	data, err := readAll(path)
	if err != nil {
		return Schema{}, err
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Schema{}, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Endian == "" {
		s.Endian = "big"
	}
	if strings.TrimSpace(s.Endian) == "" {
		s.Endian = "big"
	}
	if s.Size == (SizeSpec{}) {
		s.Size = SizeSpec{Auto: true}
	}
	if err := s.Normalize(defaultDB); err != nil {
		return Schema{}, err
	}
	return s, nil
}

func LoadRaw(path string, defaultDB *int) (Schema, error) {
	data, err := readAll(path)
	if err != nil {
		return Schema{}, err
	}
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Schema{}, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Endian == "" {
		s.Endian = "big"
	}
	if strings.TrimSpace(s.Endian) == "" {
		s.Endian = "big"
	}
	if s.Size == (SizeSpec{}) {
		s.Size = SizeSpec{Auto: true}
	}
	if s.DB == 0 && defaultDB != nil {
		s.DB = *defaultDB
	}
	return s, nil
}

func (s *Schema) Normalize(defaultDB *int) error {
	s.Endian = strings.ToLower(strings.TrimSpace(s.Endian))
	if s.Endian == "" {
		s.Endian = "big"
	}
	if s.DB == 0 && defaultDB != nil {
		s.DB = *defaultDB
	}
	db := s.DB
	for i := range s.Tags {
		a, err := address.Parse(s.Tags[i].Addr, &db)
		if err != nil {
			return fmt.Errorf("tag %d: %w", i+1, err)
		}
		s.Tags[i].Addr = a.Canonical()
		s.Tags[i].Type = normalizeType(s.Tags[i].Type)
		s.Tags[i].Role = strings.ToLower(strings.TrimSpace(s.Tags[i].Role))
		if s.Tags[i].Heartbeat != nil && s.Tags[i].Role == "" {
			s.Tags[i].Role = "heartbeat"
		}
		if s.Tags[i].Role == "heartbeat" {
			if s.Tags[i].Heartbeat == nil {
				s.Tags[i].Heartbeat = &HeartbeatSpec{}
			}
			if strings.TrimSpace(s.Tags[i].Heartbeat.Interval) == "" {
				s.Tags[i].Heartbeat.Interval = "1s"
			}
			if strings.TrimSpace(s.Tags[i].Heartbeat.Timeout) == "" {
				s.Tags[i].Heartbeat.Timeout = "5s"
			}
			if strings.TrimSpace(s.Tags[i].Heartbeat.Polarity) == "" {
				s.Tags[i].Heartbeat.Polarity = "set-true"
			}
		}
		if s.Tags[i].Heartbeat != nil {
			s.Tags[i].Heartbeat.Polarity = strings.ToLower(strings.TrimSpace(s.Tags[i].Heartbeat.Polarity))
		}
		if s.Tags[i].UI != nil {
			s.Tags[i].UI.Widget = strings.ToLower(strings.TrimSpace(s.Tags[i].UI.Widget))
			s.Tags[i].UI.Mode = strings.ToLower(strings.TrimSpace(s.Tags[i].UI.Mode))
			s.Tags[i].UI.Group = strings.TrimSpace(s.Tags[i].UI.Group)
			s.Tags[i].UI.Color = strings.ToLower(strings.TrimSpace(s.Tags[i].UI.Color))
			s.Tags[i].UI.Unit = strings.TrimSpace(s.Tags[i].UI.Unit)
		}
	}
	return nil
}

func normalizeType(t string) string {
	t = strings.TrimSpace(t)
	up := strings.ToUpper(t)
	if strings.HasPrefix(up, "STRING[") {
		return up
	}
	return up
}

func Save(path string, s Schema) error {
	if err := s.Normalize(&s.DB); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(s); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if path == "-" {
		_, err := os.Stdout.Write(buf.Bytes())
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func readAll(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func Merge(base, incoming Schema, strict bool) (Schema, error) {
	out := base
	if incoming.Version != 0 {
		out.Version = incoming.Version
	}
	if incoming.DB != 0 {
		out.DB = incoming.DB
	}
	if incoming.Name != "" {
		out.Name = incoming.Name
	}
	if incoming.Endian != "" {
		out.Endian = incoming.Endian
	}
	out.Optimized = incoming.Optimized
	if incoming.Size != (SizeSpec{}) {
		out.Size = incoming.Size
	}
	idx := map[string]int{}
	for i, t := range out.Tags {
		idx[t.Addr] = i
	}
	for _, t := range incoming.Tags {
		if i, ok := idx[t.Addr]; ok {
			if strict {
				return Schema{}, fmt.Errorf("conflicting tag at %s", t.Addr)
			}
			out.Tags[i] = t
			continue
		}
		out.Tags = append(out.Tags, t)
	}
	return out, nil
}

func FindTag(s Schema, key string, defaultDB *int) (int, bool) {
	for i, t := range s.Tags {
		if strings.EqualFold(t.Name, key) {
			return i, true
		}
	}
	if addr, err := address.Parse(key, defaultDB); err == nil {
		c := addr.Canonical()
		for i, t := range s.Tags {
			if t.Addr == c {
				return i, true
			}
		}
	}
	return -1, false
}

func ParseExternal(path string, defaultDB *int) (Schema, error) {
	data, err := readAll(path)
	if err != nil {
		return Schema{}, err
	}
	var s Schema
	if json.Unmarshal(data, &s) == nil {
		if s.Version == 0 {
			s.Version = 1
		}
		if s.Size == (SizeSpec{}) {
			s.Size = SizeSpec{Auto: true}
		}
		if err := s.Normalize(defaultDB); err != nil {
			return Schema{}, err
		}
		return s, nil
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Schema{}, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Size == (SizeSpec{}) {
		s.Size = SizeSpec{Auto: true}
	}
	if err := s.Normalize(defaultDB); err != nil {
		return Schema{}, err
	}
	return s, nil
}

func EnsureParentDir(path string) error {
	if path == "-" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func Validate(s Schema, strict bool) ([]Diagnostic, error) {
	var diags []Diagnostic
	for _, t := range s.Tags {
		switch t.Role {
		case "", "heartbeat":
		default:
			msg := fmt.Sprintf("tag %s has unknown role %q", t.Addr, t.Role)
			if strict {
				return nil, fmt.Errorf("%s", msg)
			}
			diags = append(diags, Diagnostic{Level: "warning", Message: msg})
		}
		if t.UI != nil {
			if t.UI.Min != nil && t.UI.Max != nil && *t.UI.Min > *t.UI.Max {
				return nil, fmt.Errorf("tag %s ui.min (%v) must be <= ui.max (%v)", t.Addr, *t.UI.Min, *t.UI.Max)
			}
			if t.UI.Step != nil && *t.UI.Step <= 0 {
				return nil, fmt.Errorf("tag %s ui.step (%v) must be > 0", t.Addr, *t.UI.Step)
			}
		}
		if t.Role != "heartbeat" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(t.Type), "BOOL") {
			msg := fmt.Sprintf("tag %s role=heartbeat requires type BOOL", t.Addr)
			if strict {
				return nil, fmt.Errorf("%s", msg)
			}
			diags = append(diags, Diagnostic{Level: "warning", Message: msg})
		}
		hb := t.Heartbeat
		if hb == nil {
			hb = &HeartbeatSpec{Interval: "1s", Timeout: "5s", Polarity: "set-true"}
		}
		interval, err := time.ParseDuration(strings.TrimSpace(hb.Interval))
		if err != nil {
			return nil, fmt.Errorf("tag %s invalid heartbeat.interval: %w", t.Addr, err)
		}
		timeout, err := time.ParseDuration(strings.TrimSpace(hb.Timeout))
		if err != nil {
			return nil, fmt.Errorf("tag %s invalid heartbeat.timeout: %w", t.Addr, err)
		}
		if interval >= timeout {
			msg := fmt.Sprintf("tag %s heartbeat interval (%s) must be < timeout (%s)", t.Addr, interval, timeout)
			if strict {
				return nil, fmt.Errorf("%s", msg)
			}
			diags = append(diags, Diagnostic{Level: "warning", Message: msg})
		}
		switch hb.Polarity {
		case "", "set-true", "toggle", "set-false":
		default:
			msg := fmt.Sprintf("tag %s invalid heartbeat polarity %q", t.Addr, hb.Polarity)
			if strict {
				return nil, fmt.Errorf("%s", msg)
			}
			diags = append(diags, Diagnostic{Level: "warning", Message: msg})
		}
	}
	return diags, nil
}
