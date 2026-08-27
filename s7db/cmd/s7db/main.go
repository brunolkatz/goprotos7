package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/brunolkatz/goprotos7/s7db/internal/address"
	"github.com/brunolkatz/goprotos7/s7db/internal/config"
	"github.com/brunolkatz/goprotos7/s7db/internal/decode"
	heartbeatpkg "github.com/brunolkatz/goprotos7/s7db/internal/heartbeat"
	"github.com/brunolkatz/goprotos7/s7db/internal/layout"
	"github.com/brunolkatz/goprotos7/s7db/internal/output"
	"github.com/brunolkatz/goprotos7/s7db/internal/pack"
	"github.com/brunolkatz/goprotos7/s7db/internal/plc"
	"github.com/brunolkatz/goprotos7/s7db/internal/schema"
	"github.com/brunolkatz/goprotos7/s7db/internal/server"
	"github.com/brunolkatz/goprotos7/s7db/internal/sim"
	"github.com/brunolkatz/goprotos7/s7db/internal/simlang"
	"github.com/brunolkatz/goprotos7/s7db/internal/version"
	"gopkg.in/yaml.v3"
)

type CLI struct {
	Version bool   `help:"Print version and exit."`
	File    string `short:"f" help:"Schema path."`
	Config  string `short:"c" help:"Global config path."`
	Timeout string `short:"t" help:"Overall timeout (e.g. 5s)."`
	Quiet   bool   `short:"q" help:"Quiet mode."`
	Verbose bool   `short:"v" help:"Verbose mode."`
	NoColor bool   `help:"Disable colors (also honors NO_COLOR and TERM=dumb)."`
	DryRun  bool   `help:"Print actions and do not write anything."`
	Force   bool   `help:"Overwrite or replace without confirmation."`
	Strict  bool   `help:"Fail on overlaps and non-canonical addresses."`
	Endian  string `help:"Default endian (big|little)."`
	DB      *int   `help:"Default DB number for short addresses."`

	Root      RootCmd      `cmd:"" hidden:"" default:"1" name:"__root"`
	Init      InitCmd      `cmd:"" help:"Create a new schema file."`
	Reset     ResetCmd     `cmd:"" help:"Reset schema file (wipe and optionally re-import)."`
	Add       AddCmd       `cmd:"" help:"Add one or more tags."`
	Set       SetCmd       `cmd:"" help:"Update a tag by address or name."`
	Remove    RemoveCmd    `cmd:"" name:"remove" aliases:"rm" help:"Remove tags by address or name."`
	List      ListCmd      `cmd:"" help:"List tags from schema."`
	Pack      PackCmd      `cmd:"" help:"Pack schema into a DB binary image."`
	Unpack    UnpackCmd    `cmd:"" help:"Unpack a binary image back to a schema."`
	Check     CheckCmd     `cmd:"" help:"Validate schema layout and types."`
	Info      InfoCmd      `cmd:"" help:"Print schema summary."`
	Watch     WatchCmd     `cmd:"" help:"Watch live PLC values (raw bytes + typed decode)."`
	Heartbeat HeartbeatCmd `cmd:"" help:"Write a PLC heartbeat bit from OS service."`
	Serve     ServeCmd     `cmd:"" help:"Serve SCADA HMI and WebSocket API."`
	Compile   CompileCmd   `cmd:"" help:"Compile s7sim script (parse + typecheck)."`
	Sim       SimCmd       `cmd:"" help:"Run s7sim script in offline image or PLC-assisted mode."`
}

type InitLikeFlags struct {
	From      string `name:"from" help:"Import YAML/JSON schema from path."`
	Merge     bool   `help:"Merge imported schema into current schema."`
	Name      string `help:"Symbolic DB name."`
	Size      string `default:"auto" help:"Schema size in bytes or auto."`
	Optimized bool   `help:"Mark optimized DB."`
}

type InitCmd struct {
	InitLikeFlags
}

type ResetCmd struct {
	InitLikeFlags
}

type AddCmd struct {
	Type string   `name:"type" short:"T" help:"Tag type or comma-separated list."`
	Name string   `short:"n" help:"Tag name."`
	Desc string   `short:"d" help:"Tag description."`
	Init string   `help:"Initial value."`
	From string   `help:"Import tags from CSV or YAML list."`
	Args []string `arg:"" optional:"" name:"addr" help:"One or more addresses."`
}

type SetCmd struct {
	Name string `short:"n" help:"New tag name."`
	Desc string `short:"d" help:"New description."`
	Init string `help:"New init value."`
	Type string `short:"T" help:"New type."`
	Ref  string `arg:"" required:"" name:"tag" help:"Address or existing tag name."`
}

type RemoveCmd struct {
	Items []string `arg:"" name:"tag" help:"Addresses or tag names to remove."`
}

type ListCmd struct {
	Output  string `short:"o" enum:"table,json,csv" default:"table" help:"Output format."`
	Columns string `help:"Comma-separated columns."`
	Addr    string `help:"Filter one address or tag name."`
}

type PackCmd struct {
	Format string `enum:"raw,mc7" default:"raw" help:"Pack format."`
	Fill   string `default:"0x00" help:"Gap fill byte (hex)."`
	Pad    int    `default:"0" help:"Pad final size to N bytes."`
	Output string `short:"o" default:"DB.bin" help:"Output path or - for stdout."`
}

type UnpackCmd struct {
	From   string `help:"Template schema path."`
	Input  string `arg:"" required:"" name:"bin" help:"Input binary path or - for stdin."`
	Output string `short:"o" default:"-" help:"Output schema path or - for stdout."`
}

type CheckCmd struct{}

type InfoCmd struct{}

type WatchCmd struct {
	Addr      string   `name:"addr" aliases:"ip" help:"PLC host/IP."`
	Rack      *int     `help:"PLC rack (default 0)."`
	Slot      *int     `help:"PLC slot (default 1)."`
	Port      *int     `help:"PLC port (default 102)."`
	Interval  string   `short:"i" default:"500ms" help:"Poll interval."`
	Count     int      `default:"0" help:"Stop after N samples (0 forever)."`
	Vars      string   `help:"Comma-separated addresses or tag names (names use schema type; DBX/DBB/DBW/DBD infer BOOL/BYTE/WORD/DWORD)."`
	Once      bool     `help:"Read once and exit."`
	Diff      bool     `help:"Only print changed values."`
	JSON      bool     `help:"Emit one JSON object per sample."`
	Reconnect bool     `help:"Auto reconnect with exponential backoff."`
	Output    string   `short:"o" enum:"table,json" default:"table" help:"Output format."`
	Args      []string `arg:"" optional:"" name:"var" help:"Addresses or tag names (names use schema type; DBX/DBB/DBW/DBD infer BOOL/BYTE/WORD/DWORD)."`
}

type HeartbeatCmd struct {
	Addr             string `name:"addr" aliases:"ip" help:"PLC host/IP."`
	Rack             *int   `help:"PLC rack (default 0)."`
	Slot             *int   `help:"PLC slot (default 1)."`
	Port             *int   `help:"PLC port (default 102)."`
	Interval         string `short:"i" help:"Write interval (default schema heartbeat.interval or 1s)."`
	HeartbeatTimeout string `name:"heartbeat-timeout" help:"PLC heartbeat policy timeout (default schema heartbeat.timeout or 5s)."`
	Mode             string `help:"Beat mode (set-true|toggle)."`
	Tag              string `help:"Tag name alternative to operand."`
	Count            int    `default:"0" help:"Stop after N beats (0 forever)."`
	Once             bool   `help:"Single write then exit."`
	NoReadback       bool   `help:"Skip read-after-write."`
	RequireClear     bool   `help:"After set-true, require PLC clear within --clear-within."`
	ClearWithin      string `help:"Clear verification window (default interval)."`
	JSON             bool   `help:"Emit NDJSON events on stdout."`
	Reconnect        bool   `help:"Auto reconnect with exponential backoff."`
	Ref              string `arg:"" optional:"" name:"addr-or-name" help:"Heartbeat target address or tag name."`
}

type ServeCmd struct {
	Listen    string `default:"127.0.0.1:8080" help:"Listen address host:port."`
	Token     string `help:"Bearer token for write operations."`
	ReadOnly  bool   `help:"Disable all write operations."`
	Interval  string `short:"i" default:"200ms" help:"PLC polling interval for WS updates."`
	Addr      string `name:"addr" aliases:"ip" help:"PLC host/IP."`
	Rack      *int   `help:"PLC rack (default 0)."`
	Slot      *int   `help:"PLC slot (default 1)."`
	Port      *int   `help:"PLC port (default 102)."`
	Heartbeat bool   `help:"Run embedded heartbeat loop for role=heartbeat tag."`
	NoOpen    bool   `help:"Do not print listen URL."`
}

type CompileCmd struct {
	Path string `arg:"" required:"" name:"file.sim" help:"Script path (.sim)."`
}

type SimCmd struct {
	Path       string `arg:"" required:"" name:"file.sim" help:"Script path (.sim)."`
	Duration   string `default:"10s" help:"Run duration (0 = until interrupt)."`
	Cycles     int    `default:"0" help:"Stop after N cycles (0 = unlimited)."`
	Rate       string `help:"Override script tick duration."`
	Once       bool   `help:"Run exactly one cycle."`
	Trace      bool   `help:"Print changes each cycle."`
	Output     string `short:"o" enum:"table,json" default:"table" help:"Trace output format."`
	Watch      string `help:"Comma-separated names to trace."`
	SeedJSON   string `help:"Initial values JSON file ({\"Tag\":value})."`
	Offline    bool   `help:"Force offline mode."`
	PLC        bool   `help:"Enable PLC I/O mode (default when omitted is offline)."`
	Addr       string `name:"addr" aliases:"ip" help:"PLC host/IP."`
	Rack       *int   `help:"PLC rack (default 0)."`
	Slot       *int   `help:"PLC slot (default 1)."`
	Port       *int   `help:"PLC port (default 102)."`
	ReadOnly   bool   `help:"With --plc: pull only, do not push writes."`
	Write      bool   `help:"With --plc: enable pushes (requires --allow-write or --write-all)."`
	AllowWrite string `help:"Comma-separated writable tag names whitelist."`
	WriteAll   bool   `help:"With --plc --write: allow every schema tag except heartbeat (unless --take-heartbeat)."`
	TakeHB     bool   `name:"take-heartbeat" help:"Allow writes to tags with role=heartbeat."`
	Reconnect  bool   `help:"Reconnect on pull/push errors."`
}

type App struct {
	CLI     *CLI
	Config  config.Config
	Now     func() time.Time
	Color   bool
	Timeout time.Duration
}

type RootCmd struct{}

func (c *RootCmd) Run(*App) error {
	return usagef("missing command")
}

func main() {
	os.Exit(run())
}

func run() int {
	cli := &CLI{}
	parser, err := kong.New(cli, kong.Name("s7db"), kong.UsageOnError(), kong.Description("Manage Siemens S7 DB schemas and binary images."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "s7db: %v\n", err)
		return 2
	}
	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "s7db: %v\n", err)
		return 2
	}
	if cli.Version {
		fmt.Println(version.Value)
		return 0
	}
	app, err := buildApp(cli)
	if err != nil {
		fmt.Fprintf(os.Stderr, "s7db: %v\n", err)
		return 2
	}
	if err := kctx.Run(app); err != nil {
		code := 1
		var ue *UsageError
		if errors.As(err, &ue) {
			code = 2
		}
		fmt.Fprintf(os.Stderr, "s7db: %v\n", err)
		return code
	}
	return 0
}

func buildApp(cli *CLI) (*App, error) {
	cfgPath := cli.Config
	if cfgPath == "" {
		cfgPath = config.DefaultPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	if cli.File == "" {
		cli.File = cfg.File
	}
	if cli.File == "" {
		cli.File = "./s7db.yml"
	}
	if cli.DB == nil {
		cli.DB = cfg.DB
	}
	if cli.Endian == "" {
		cli.Endian = cfg.Endian
	}
	if cli.Endian == "" {
		cli.Endian = "big"
	}
	if cli.Endian != "big" && cli.Endian != "little" {
		return nil, usagef("invalid --endian %q (expected big|little)", cli.Endian)
	}

	timeout := 10 * time.Second
	if cfgT, err := cfg.TimeoutDuration(); err == nil && cfgT > 0 {
		timeout = cfgT
	}
	if strings.TrimSpace(cli.Timeout) != "" {
		timeout, err = time.ParseDuration(cli.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
	}
	color := !cli.NoColor && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	return &App{
		CLI:     cli,
		Config:  cfg,
		Now:     time.Now,
		Color:   color,
		Timeout: timeout,
	}, nil
}

type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

func (c *InitCmd) Run(app *App) error  { return runInitLike(app, c.InitLikeFlags, false) }
func (c *ResetCmd) Run(app *App) error { return runInitLike(app, c.InitLikeFlags, true) }

func runInitLike(app *App, flags InitLikeFlags, isReset bool) error {
	path := app.CLI.File
	exists, err := schema.Exists(path)
	if err != nil {
		return err
	}
	if exists && !app.CLI.Force {
		return usagef("%s exists, use --force", path)
	}
	var out schema.Schema
	if flags.From != "" {
		in, err := schema.ParseExternal(flags.From, app.CLI.DB)
		if err != nil {
			return err
		}
		out = in
	} else {
		db := valueIntPtrDefault(0, app.CLI.DB)
		if db == 0 {
			return usagef("--db is required when --from is not provided")
		}
		out = schema.New(db)
	}
	if flags.Merge {
		base := out
		if exists && !isReset {
			base, err = schema.Load(path, app.CLI.DB)
			if err != nil {
				return err
			}
		}
		out, err = schema.Merge(base, out, app.CLI.Strict)
		if err != nil {
			return err
		}
	}
	if flags.Name != "" {
		out.Name = flags.Name
	}
	out.Optimized = flags.Optimized
	size, err := parseSize(flags.Size)
	if err != nil {
		return err
	}
	out.Size = size
	if out.Endian == "" {
		out.Endian = app.CLI.Endian
	}
	if err := out.Normalize(&out.DB); err != nil {
		return err
	}
	if app.CLI.DryRun {
		return schema.Save("-", out)
	}
	if err := schema.EnsureParentDir(path); err != nil {
		return err
	}
	return schema.Save(path, out)
}

func (c *AddCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	toAdd, err := c.buildTags(app, s.DB)
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, t := range s.Tags {
		idx[t.Addr] = i
	}
	for _, t := range toAdd {
		if i, ok := idx[t.Addr]; ok {
			if !app.CLI.Force {
				return usagef("tag %s already exists (use --force to overwrite)", t.Addr)
			}
			s.Tags[i] = t
			continue
		}
		s.Tags = append(s.Tags, t)
	}
	if app.CLI.DryRun {
		return schema.Save("-", s)
	}
	return schema.Save(app.CLI.File, s)
}

func (c *AddCmd) buildTags(app *App, db int) ([]schema.Tag, error) {
	if c.From != "" {
		return loadTagsFromFile(c.From, db, app.CLI.DB)
	}
	if len(c.Args) == 0 {
		return nil, usagef("at least one address or --from is required")
	}
	if c.Type == "" {
		return nil, usagef("--type is required when adding from arguments")
	}
	types := splitCSV(c.Type)
	if len(types) != 1 && len(types) != len(c.Args) {
		return nil, usagef("--type must be one value or match address count")
	}
	tags := make([]schema.Tag, 0, len(c.Args))
	for i, raw := range c.Args {
		typ := types[0]
		if len(types) == len(c.Args) {
			typ = types[i]
		}
		addrValue, err := address.Parse(raw, valueOrDB(app.CLI.DB, db))
		if err != nil {
			return nil, err
		}
		tag := schema.Tag{
			Addr: addrValue.Canonical(),
			Type: strings.ToUpper(strings.TrimSpace(typ)),
			Name: c.Name,
			Desc: c.Desc,
		}
		if c.Init != "" {
			tag.Init = c.Init
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (c *SetCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	i, ok := schema.FindTag(s, c.Ref, app.CLI.DB)
	if !ok {
		return usagef("tag %q not found", c.Ref)
	}
	tag := s.Tags[i]
	if c.Name != "" {
		tag.Name = c.Name
	}
	if c.Desc != "" {
		tag.Desc = c.Desc
	}
	if c.Init != "" {
		tag.Init = c.Init
	}
	if c.Type != "" {
		if !app.CLI.Force && !strings.EqualFold(tag.Type, c.Type) {
			return usagef("type change requires --force")
		}
		tag.Type = strings.ToUpper(strings.TrimSpace(c.Type))
	}
	s.Tags[i] = tag
	if app.CLI.DryRun {
		return schema.Save("-", s)
	}
	return schema.Save(app.CLI.File, s)
}

func (c *RemoveCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	if len(c.Items) == 0 {
		return usagef("at least one tag operand is required")
	}
	keep := make([]schema.Tag, 0, len(s.Tags))
	for _, t := range s.Tags {
		if containsRef(c.Items, t, app.CLI.DB) {
			continue
		}
		keep = append(keep, t)
	}
	removed := len(s.Tags) - len(keep)
	if removed == 0 && app.CLI.Strict {
		return usagef("no tags removed")
	}
	s.Tags = keep
	if app.CLI.DryRun {
		return schema.Save("-", s)
	}
	return schema.Save(app.CLI.File, s)
}

func (c *ListCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	rows, err := rowsFromSchema(s)
	if err != nil {
		return err
	}
	if c.Addr != "" {
		rows = filterRows(rows, c.Addr, app.CLI.DB)
	}
	cols := []string{"addr", "name", "type", "role", "hb_interval", "hb_timeout", "offset", "size", "init", "desc"}
	if strings.TrimSpace(c.Columns) != "" {
		cols = splitCSV(c.Columns)
	}
	switch c.Output {
	case "table":
		return output.WriteTable(os.Stdout, rows, cols, app.Color)
	case "json":
		return output.WriteJSON(os.Stdout, rows)
	case "csv":
		return output.WriteCSV(os.Stdout, rows, cols)
	default:
		return usagef("invalid output %q", c.Output)
	}
}

func (c *PackCmd) Run(app *App) error {
	if c.Format == "mc7" {
		return fmt.Errorf("mc7 format not implemented yet")
	}
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	fill, err := parseHexByte(c.Fill)
	if err != nil {
		return err
	}
	payload, computedSize, err := pack.Pack(s, fill, c.Pad, app.CLI.Strict)
	if err != nil {
		return err
	}
	if !app.CLI.Quiet {
		fmt.Fprintf(os.Stderr, "final size: %d bytes (computed %d)\n", len(payload), computedSize)
	}
	if app.CLI.DryRun {
		_, err := os.Stdout.Write(payload)
		return err
	}
	return writeBytes(c.Output, payload)
}

func (c *UnpackCmd) Run(app *App) error {
	data, err := readBytes(c.Input)
	if err != nil {
		return err
	}
	db := valueIntPtrDefault(1, app.CLI.DB)
	var template *schema.Schema
	if c.From != "" {
		s, err := schema.Load(c.From, app.CLI.DB)
		if err != nil {
			return err
		}
		template = &s
		if s.DB != 0 {
			db = s.DB
		}
	}
	out, err := pack.Unpack(data, db, template)
	if err != nil {
		return err
	}
	return schema.Save(c.Output, out)
}

func (c *CheckCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	res, err := layout.Build(s, app.CLI.Strict)
	if err != nil {
		return usagef("%s", err.Error())
	}
	diags, err := schema.Validate(s, app.CLI.Strict)
	if err != nil {
		return usagef("%s", err.Error())
	}
	for _, d := range res.Diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s\n", d.Level, d.Message)
	}
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "%s: %s\n", d.Level, d.Message)
	}
	if app.CLI.Strict && (len(res.Diagnostics) > 0 || len(diags) > 0) {
		return usagef("strict check failed")
	}
	return nil
}

func (c *InfoCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	res, err := layout.Build(s, false)
	if err != nil {
		return err
	}
	summary := map[string]any{
		"db":            s.DB,
		"name":          s.Name,
		"endian":        s.Endian,
		"optimized":     s.Optimized,
		"computed_size": res.ComputedSize,
		"final_size":    res.FinalSize,
		"tag_count":     len(s.Tags),
		"schema_path":   app.CLI.File,
	}
	heartbeatTags := []map[string]any{}
	for _, t := range s.Tags {
		if t.Role != "heartbeat" || t.Heartbeat == nil {
			continue
		}
		heartbeatTags = append(heartbeatTags, map[string]any{
			"addr":     t.Addr,
			"name":     t.Name,
			"interval": t.Heartbeat.Interval,
			"timeout":  t.Heartbeat.Timeout,
			"polarity": t.Heartbeat.Polarity,
		})
	}
	if len(heartbeatTags) > 0 {
		summary["heartbeat_tags"] = heartbeatTags
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

func (c *HeartbeatCmd) Run(app *App) error {
	s, _ := schema.Load(app.CLI.File, app.CLI.DB)
	target, err := resolveHeartbeatTarget(s, c.Ref, c.Tag, app.CLI.DB)
	if err != nil {
		return usagef("heartbeat: %v", err)
	}

	if !strings.EqualFold(target.Tag.Type, "BOOL") {
		if app.CLI.Strict && !app.CLI.Force {
			return usagef("heartbeat: tag %s must be BOOL", target.Address.Canonical())
		}
		if !app.CLI.Quiet {
			fmt.Fprintf(os.Stderr, "warning: heartbeat tag %s is type %s (expected BOOL)\n", target.Address.Canonical(), target.Tag.Type)
		}
	}

	hb := target.Tag.Heartbeat
	if hb == nil {
		hb = &schema.HeartbeatSpec{Interval: "1s", Timeout: "5s", Polarity: "set-true"}
	}
	intervalRaw := hb.Interval
	if c.Interval != "" {
		intervalRaw = c.Interval
	}
	timeoutRaw := hb.Timeout
	if c.HeartbeatTimeout != "" {
		timeoutRaw = c.HeartbeatTimeout
	}
	mode := hb.Polarity
	if mode == "" {
		mode = "set-true"
	}
	if c.Mode != "" {
		mode = c.Mode
	}
	if mode != "set-true" && mode != "toggle" {
		return usagef("heartbeat: invalid mode %q (expected set-true|toggle)", mode)
	}
	interval, err := time.ParseDuration(intervalRaw)
	if err != nil {
		return usagef("heartbeat: invalid interval %q", intervalRaw)
	}
	timeoutVal, err := time.ParseDuration(timeoutRaw)
	if err != nil {
		return usagef("heartbeat: invalid timeout %q", timeoutRaw)
	}
	if interval >= timeoutVal && !app.CLI.Force {
		return usagef("heartbeat: interval (%s) must be < timeout (%s)", interval, timeoutVal)
	}
	if interval >= timeoutVal && !app.CLI.Quiet {
		fmt.Fprintf(os.Stderr, "warning: heartbeat interval (%s) >= timeout (%s)\n", interval, timeoutVal)
	}
	clearWithin := interval
	if c.ClearWithin != "" {
		clearWithin, err = time.ParseDuration(c.ClearWithin)
		if err != nil {
			return usagef("heartbeat: invalid --clear-within %q", c.ClearWithin)
		}
	}

	addr := c.Addr
	if addr == "" {
		addr = app.Config.PLC.Addr
	}
	if addr == "" && !app.CLI.DryRun {
		return usagef("heartbeat: --addr is required")
	}
	rack := valueIntPtrDefault(0, c.Rack, app.Config.PLC.Rack)
	slot := valueIntPtrDefault(1, c.Slot, app.Config.PLC.Slot)
	port := valueIntPtrDefault(102, c.Port, app.Config.PLC.Port)

	var client heartbeatpkg.Client
	if !app.CLI.DryRun {
		client = plc.New(addr, rack, slot, port, app.Timeout)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return heartbeatpkg.Run(sigCtx, client, heartbeatpkg.Config{
		Address:      target.Address,
		Name:         target.Tag.Name,
		Interval:     interval,
		Timeout:      timeoutVal,
		Mode:         mode,
		Count:        c.Count,
		Once:         c.Once,
		NoReadback:   c.NoReadback,
		RequireClear: c.RequireClear,
		ClearWithin:  clearWithin,
		Reconnect:    c.Reconnect,
		Strict:       app.CLI.Strict,
		Quiet:        app.CLI.Quiet,
		JSON:         c.JSON,
		DryRun:       app.CLI.DryRun,
		Now:          app.Now,
		Out:          os.Stdout,
		Err:          os.Stderr,
	})
}

func (c *ServeCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	diags, err := schema.Validate(s, app.CLI.Strict)
	if err != nil {
		return usagef("serve: %s", err.Error())
	}
	for _, d := range diags {
		fmt.Fprintf(os.Stderr, "%s: %s\n", d.Level, d.Message)
	}

	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		return usagef("serve: invalid interval %q", c.Interval)
	}
	addr := c.Addr
	if addr == "" {
		addr = app.Config.PLC.Addr
	}
	if addr == "" {
		return usagef("serve: --addr is required")
	}
	rack := valueIntPtrDefault(0, c.Rack, app.Config.PLC.Rack)
	slot := valueIntPtrDefault(1, c.Slot, app.Config.PLC.Slot)
	port := valueIntPtrDefault(102, c.Port, app.Config.PLC.Port)

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return server.Run(sigCtx, s, server.Config{
		Listen:    c.Listen,
		Token:     c.Token,
		ReadOnly:  c.ReadOnly,
		Interval:  interval,
		Addr:      addr,
		Rack:      rack,
		Slot:      slot,
		Port:      port,
		Heartbeat: c.Heartbeat,
		NoOpen:    c.NoOpen,
		Force:     app.CLI.Force,
		Timeout:   app.Timeout,
		Now:       app.Now,
		Out:       os.Stdout,
		Err:       os.Stderr,
	})
}

func (c *CompileCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	srcBytes, err := os.ReadFile(c.Path)
	if err != nil {
		return err
	}
	prog, diags := simlang.Compile(c.Path, string(srcBytes), s)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(os.Stderr, d.String())
		}
		return usagef("compile failed")
	}
	for _, d := range prog.Warnings {
		fmt.Fprintln(os.Stderr, d.String())
	}
	fmt.Fprintf(os.Stdout, "ok: %d statements\n", prog.Statements)
	return nil
}

func (c *SimCmd) Run(app *App) error {
	s, err := schema.Load(app.CLI.File, app.CLI.DB)
	if err != nil {
		return err
	}
	srcBytes, err := os.ReadFile(c.Path)
	if err != nil {
		return err
	}
	prog, diags := simlang.Compile(c.Path, string(srcBytes), s)
	if len(diags) > 0 {
		for _, d := range diags {
			fmt.Fprintln(os.Stderr, d.String())
		}
		return usagef("compile failed")
	}
	for _, d := range prog.Warnings {
		fmt.Fprintln(os.Stderr, d.String())
	}
	if c.WriteAll && (!c.PLC || !c.Write) {
		return usagef("sim: --write-all requires --plc --write")
	}
	if c.WriteAll && strings.TrimSpace(c.AllowWrite) != "" {
		return usagef("sim: --write-all cannot be combined with --allow-write")
	}
	if c.PLC && c.Write && strings.TrimSpace(c.AllowWrite) == "" && !c.WriteAll {
		return usagef("sim: --plc --write requires --allow-write or --write-all")
	}
	if c.Once {
		c.Cycles = 1
	}
	runDur, err := time.ParseDuration(c.Duration)
	if err != nil {
		return usagef("sim: invalid --duration %q", c.Duration)
	}
	tick := prog.Tick
	if c.Rate != "" {
		tick, err = time.ParseDuration(c.Rate)
		if err != nil {
			return usagef("sim: invalid --rate %q", c.Rate)
		}
	}
	img := sim.NewImageFromSchema(s, prog)
	if err := loadSeed(img, c.SeedJSON); err != nil {
		return err
	}
	runner := sim.NewRunner(prog, img)
	runner.Tick = tick

	readNames := make([]string, 0, len(prog.UsedTags))
	for n := range prog.UsedTags {
		readNames = append(readNames, n)
	}
	watchSet := setFromCSV(c.Watch)
	for n := range watchSet {
		readNames = append(readNames, n)
	}
	allowWrite := setFromCSV(c.AllowWrite)

	var sink sim.Sink = sim.MemorySink{}
	var plcSink *sim.PLCSink
	pushEnabled := false
	if c.PLC {
		addr := c.Addr
		if addr == "" {
			addr = app.Config.PLC.Addr
		}
		if addr == "" {
			return usagef("sim: --addr is required with --plc")
		}
		rack := valueIntPtrDefault(0, c.Rack, app.Config.PLC.Rack)
		slot := valueIntPtrDefault(1, c.Slot, app.Config.PLC.Slot)
		port := valueIntPtrDefault(102, c.Port, app.Config.PLC.Port)
		client := plc.New(addr, rack, slot, port, app.Timeout)
		connectCtx, cancel := context.WithTimeout(context.Background(), app.Timeout)
		err := client.Connect(connectCtx)
		cancel()
		if err != nil {
			return err
		}
		defer client.Close()
		plcSink, err = sim.NewPLCSink(client, s, prog, img)
		if err != nil {
			return err
		}
		if app.CLI.DryRun {
			sink = &sim.DryRunSink{Base: plcSink, Out: os.Stderr}
		} else {
			sink = plcSink
		}
		pushEnabled = c.Write
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	start := app.Now()
	cycle := 0
	for {
		if sigCtx.Err() != nil {
			break
		}
		if runDur > 0 && app.Now().Sub(start) >= runDur {
			break
		}
		if c.Cycles > 0 && cycle >= c.Cycles {
			break
		}
		if err := sink.Pull(sigCtx, readNames); err != nil {
			if c.Reconnect && plcSink != nil {
				_ = plcSink.Reconnect(sigCtx)
				continue
			}
			return fmt.Errorf("sim cycle %d pull error: %w", cycle+1, err)
		}
		changes, err := runner.Step()
		if err != nil {
			return fmt.Errorf("sim cycle %d runtime error: %w", cycle+1, err)
		}
		pushes, skippedHB := filterPushes(changes, allowWrite, s, c.TakeHB, pushEnabled, c.WriteAll)
		for _, name := range skippedHB {
			fmt.Fprintf(os.Stderr, "warning: skipped heartbeat write to %s (use --take-heartbeat to allow)\n", name)
		}
		for _, ch := range changes {
			if img.ConsumeStringTruncated(ch.Name) {
				fmt.Fprintf(os.Stderr, "warning: truncated string write for %s to max length\n", ch.Name)
			}
		}
		if pushEnabled && len(pushes) > 0 {
			if err := sink.Push(sigCtx, pushes); err != nil {
				if c.Reconnect && plcSink != nil {
					_ = plcSink.Reconnect(sigCtx)
					continue
				}
				return fmt.Errorf("sim cycle %d push error: %w", cycle+1, err)
			}
		}
		if c.Trace {
			printTrace(os.Stdout, c.Output, cycle+1, pushes, watchSet)
		}
		cycle++
		select {
		case <-sigCtx.Done():
			break
		case <-time.After(tick):
		}
	}
	if !app.CLI.Quiet {
		fmt.Fprintf(os.Stderr, "sim finished: cycles=%d tick=%s\n", cycle, tick)
	}
	return nil
}

func (c *WatchCmd) Run(app *App) error {
	vars := append([]string{}, c.Args...)
	if c.Vars != "" {
		vars = append(vars, splitCSV(c.Vars)...)
	}
	if len(vars) == 0 {
		return usagef("at least one variable is required (--vars or operands)")
	}
	addr := c.Addr
	if addr == "" {
		addr = app.Config.PLC.Addr
	}
	if addr == "" {
		return usagef("--addr is required")
	}
	rack := valueIntPtrDefault(0, c.Rack, app.Config.PLC.Rack)
	slot := valueIntPtrDefault(1, c.Slot, app.Config.PLC.Slot)
	port := valueIntPtrDefault(102, c.Port, app.Config.PLC.Port)

	interval, err := time.ParseDuration(c.Interval)
	if err != nil {
		return usagef("invalid interval: %v", err)
	}
	if c.JSON {
		c.Output = "json"
	}

	s, _ := schema.Load(app.CLI.File, app.CLI.DB)
	resolved, resolveErrs, err := resolveWatchVars(s, vars, app.CLI.DB, app.CLI.Endian)
	if err != nil {
		return usagef("watch: %v", err)
	}
	for _, resolveErr := range resolveErrs {
		fmt.Fprintf(os.Stderr, "s7db: watch: %v\n", resolveErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), app.Timeout)
	defer cancel()
	client := plc.New(addr, rack, slot, port, app.Timeout)
	if err := client.Connect(ctx); err != nil {
		if !c.Reconnect {
			return err
		}
	}
	defer client.Close()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	tick := time.NewTicker(interval)
	defer tick.Stop()

	prev := map[string]watchDiffState{}
	sampleCount := 0
	for {
		if err := c.printSample(sigCtx, app, client, resolved, prev); err != nil {
			if !c.Reconnect {
				return err
			}
			backoff := time.Second
			for {
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				rc, cancelConn := context.WithTimeout(sigCtx, app.Timeout)
				err = client.Connect(rc)
				cancelConn()
				if err == nil {
					break
				}
				if sigCtx.Err() != nil {
					return nil
				}
				fmt.Fprintf(os.Stderr, "reconnect failed: %v\n", err)
			}
			continue
		}
		sampleCount++
		if c.Once || (c.Count > 0 && sampleCount >= c.Count) {
			return nil
		}
		select {
		case <-sigCtx.Done():
			return nil
		case <-tick.C:
		}
	}
}

type watchVar struct {
	Key       string
	Name      string
	Address   address.Address
	Type      string
	Size      int
	Bit       int
	ByteOrder binary.ByteOrder
}

type heartbeatTarget struct {
	Tag     schema.Tag
	Address address.Address
}

func resolveHeartbeatTarget(s schema.Schema, ref, tagName string, defaultDB *int) (heartbeatTarget, error) {
	db := s.DB
	if db == 0 && defaultDB != nil {
		db = *defaultDB
	}
	key := strings.TrimSpace(ref)
	if key == "" {
		key = strings.TrimSpace(tagName)
	}
	if key != "" {
		if i, ok := schema.FindTag(s, key, &db); ok {
			addrValue, err := address.Parse(s.Tags[i].Addr, &db)
			if err != nil {
				return heartbeatTarget{}, err
			}
			return heartbeatTarget{Tag: s.Tags[i], Address: addrValue}, nil
		}
		addrValue, err := address.Parse(key, &db)
		if err != nil {
			return heartbeatTarget{}, err
		}
		return heartbeatTarget{
			Tag: schema.Tag{
				Addr:      addrValue.Canonical(),
				Name:      key,
				Type:      "BOOL",
				Role:      "heartbeat",
				Heartbeat: &schema.HeartbeatSpec{Interval: "1s", Timeout: "5s", Polarity: "set-true"},
			},
			Address: addrValue,
		}, nil
	}
	var candidates []schema.Tag
	for _, t := range s.Tags {
		if t.Role == "heartbeat" {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 1 {
		addrValue, err := address.Parse(candidates[0].Addr, &db)
		if err != nil {
			return heartbeatTarget{}, err
		}
		return heartbeatTarget{Tag: candidates[0], Address: addrValue}, nil
	}
	if len(candidates) == 0 {
		return heartbeatTarget{}, fmt.Errorf("no heartbeat target provided and schema has no tag with role=heartbeat")
	}
	return heartbeatTarget{}, fmt.Errorf("multiple heartbeat tags found, provide operand or --tag")
}

func resolveWatchVars(s schema.Schema, vars []string, defaultDB *int, fallbackEndian string) ([]watchVar, []error, error) {
	out := make([]watchVar, 0, len(vars))
	errs := make([]error, 0)
	db := s.DB
	if db == 0 && defaultDB != nil {
		db = *defaultDB
	}
	schemaEndian := s.Endian
	if strings.TrimSpace(schemaEndian) == "" {
		schemaEndian = "big"
	}
	schemaOrder, err := decode.ByteOrder(schemaEndian)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid schema endian %q", s.Endian)
	}
	fallbackOrder, err := decode.ByteOrder(fallbackEndian)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid --endian %q", fallbackEndian)
	}
	for _, v := range vars {
		if i, ok := findTagByName(s, v); ok {
			t := s.Tags[i]
			addrValue, err := address.Parse(t.Addr, &db)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", v, err))
				continue
			}
			ts, err := decode.ResolveSchemaType(t.Type, addrValue)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", v, err))
				continue
			}
			key := t.Name
			if key == "" {
				key = addrValue.Canonical()
			}
			out = append(out, watchVar{
				Key:       key,
				Name:      t.Name,
				Address:   addrValue,
				Type:      strings.ToUpper(strings.TrimSpace(t.Type)),
				Size:      ts.SizeBytes,
				Bit:       addrValue.Bit,
				ByteOrder: schemaOrder,
			})
			continue
		}
		addrValue, err := address.Parse(v, &db)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", v, err))
			continue
		}
		ts, err := decode.InferAddressType(addrValue)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", v, err))
			continue
		}
		out = append(out, watchVar{
			Key:       addrValue.Canonical(),
			Address:   addrValue,
			Type:      ts.Name,
			Size:      ts.SizeBytes,
			Bit:       addrValue.Bit,
			ByteOrder: fallbackOrder,
		})
	}
	if len(out) == 0 {
		if len(errs) == 0 {
			return nil, nil, fmt.Errorf("no variables resolved")
		}
		return nil, nil, fmt.Errorf("no variables resolved: %v", errs[0])
	}
	return out, errs, nil
}

type watchDiffState struct {
	Decoded bool
	Value   any
	RawHex  string
}

type watchSample struct {
	Addr      string `json:"addr"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	Raw       string `json:"raw"`
	RawHex    string `json:"raw_hex"`
	Value     any    `json:"value"`
	Bit       *int   `json:"bit,omitempty"`
	DecodeErr string `json:"error,omitempty"`
}

func (c *WatchCmd) printSample(ctx context.Context, app *App, client *plc.Client, vars []watchVar, prev map[string]watchDiffState) error {
	ts := app.Now().UTC().Format(time.RFC3339Nano)
	values := map[string]any{}
	samples := make([]watchSample, 0, len(vars))
	for _, v := range vars {
		raw, err := client.ReadDB(v.Address.DB, v.Address.Byte, v.Size)
		if err != nil {
			return err
		}
		rawDisplay, rawHex := rawHex(raw)

		decodedValue, err := client.Read(v.Address.Canonical(), 255)
		if err != nil {
			return err
		}

		val, _, decErr := decode.Decode(decodedValue)
		state := watchDiffState{Decoded: decErr == nil, Value: val, RawHex: rawHex}
		if c.Diff {
			if old, ok := prev[v.Key]; ok && !changedWatchValue(old, state) {
				continue
			}
		}
		if decErr != nil {
			values[v.Key] = "err"
		} else {
			values[v.Key] = val
		}
		prev[v.Key] = state
		item := watchSample{
			Addr:   v.Address.Canonical(),
			Name:   v.Name,
			Type:   v.Type,
			Raw:    rawDisplay,
			RawHex: rawHex,
			Value:  val,
		}
		if v.Type == "BOOL" {
			bit := v.Bit
			item.Bit = &bit
		}
		if decErr != nil {
			item.Value = "err"
			item.DecodeErr = decErr.Error()
		}
		samples = append(samples, item)
	}
	if len(samples) == 0 && c.Diff {
		return nil
	}
	if c.Output == "json" {
		payload := map[string]any{
			"ts":      ts,
			"values":  values,
			"samples": samples,
		}
		return json.NewEncoder(os.Stdout).Encode(payload)
	}
	for _, item := range samples {
		ref := item.Addr
		if item.Name != "" {
			ref = item.Name
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%v\n", ts, ref, item.Type, item.Raw, item.Value)
	}
	select {
	case <-ctx.Done():
		return nil
	default:
		return nil
	}
}

func findTagByName(s schema.Schema, key string) (int, bool) {
	needle := strings.TrimSpace(key)
	if needle == "" {
		return -1, false
	}
	for i, t := range s.Tags {
		if strings.EqualFold(strings.TrimSpace(t.Name), needle) {
			return i, true
		}
	}
	return -1, false
}

func changedWatchValue(old watchDiffState, current watchDiffState) bool {
	if old.Decoded && current.Decoded {
		return fmt.Sprintf("%v", old.Value) != fmt.Sprintf("%v", current.Value)
	}
	return old.RawHex != current.RawHex
}

func rawHex(raw []byte) (string, string) {
	if len(raw) == 0 {
		return "", ""
	}
	parts := make([]string, 0, len(raw))
	compact := strings.Builder{}
	for _, b := range raw {
		h := fmt.Sprintf("%02X", b)
		parts = append(parts, h)
		compact.WriteString(h)
	}
	return strings.Join(parts, " "), compact.String()
}

func rowsFromSchema(s schema.Schema) ([]output.TagRow, error) {
	res, err := layout.Build(s, false)
	if err != nil {
		return nil, err
	}
	rows := make([]output.TagRow, 0, len(res.Tags))
	for _, t := range res.Tags {
		offset := strconv.Itoa(t.Address.Byte)
		if t.Type.IsBool() {
			offset = fmt.Sprintf("%d.%d", t.Address.Byte, t.Address.Bit)
		}
		rows = append(rows, output.TagRow{
			Addr:              t.Address.Canonical(),
			Name:              t.Tag.Name,
			Type:              t.Type.Name,
			Role:              t.Tag.Role,
			HeartbeatInterval: heartbeatInterval(t.Tag),
			HeartbeatTimeout:  heartbeatTimeout(t.Tag),
			Offset:            offset,
			Size:              t.Type.SizeBytes,
			Init:              t.Tag.Init,
			Desc:              t.Tag.Desc,
		})
	}
	slices.SortFunc(rows, func(a, b output.TagRow) int { return strings.Compare(a.Addr, b.Addr) })
	return rows, nil
}

func parseSize(in string) (schema.SizeSpec, error) {
	raw := strings.TrimSpace(strings.ToLower(in))
	if raw == "" || raw == "auto" {
		return schema.SizeSpec{Auto: true}, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return schema.SizeSpec{}, usagef("invalid --size %q", in)
	}
	return schema.SizeSpec{Auto: false, Bytes: v}, nil
}

func parseHexByte(in string) (byte, error) {
	raw := strings.TrimSpace(strings.ToLower(in))
	if strings.HasPrefix(raw, "0x") {
		raw = raw[2:]
	}
	v, err := strconv.ParseUint(raw, 16, 8)
	if err != nil {
		return 0, usagef("invalid --fill byte %q", in)
	}
	return byte(v), nil
}

func splitCSV(in string) []string {
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func valueIntPtr(values ...*int) int {
	for _, v := range values {
		if v != nil {
			return *v
		}
	}
	return 0
}

func valueIntPtrDefault(def int, values ...*int) int {
	for _, v := range values {
		if v != nil {
			return *v
		}
	}
	return def
}

func valueOrDB(ptr *int, db int) *int {
	if ptr != nil {
		return ptr
	}
	if db > 0 {
		return &db
	}
	return nil
}

func readBytes(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func writeBytes(path string, payload []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(payload)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func filterRows(rows []output.TagRow, ref string, defaultDB *int) []output.TagRow {
	lookup := strings.ToLower(strings.TrimSpace(ref))
	var canon string
	if a, err := address.Parse(ref, defaultDB); err == nil {
		canon = strings.ToLower(a.Canonical())
	}
	out := make([]output.TagRow, 0, len(rows))
	for _, r := range rows {
		if strings.ToLower(r.Name) == lookup || strings.ToLower(r.Addr) == canon {
			out = append(out, r)
		}
	}
	return out
}

func containsRef(items []string, t schema.Tag, defaultDB *int) bool {
	for _, it := range items {
		if strings.EqualFold(it, t.Name) {
			return true
		}
		if a, err := address.Parse(it, defaultDB); err == nil && a.Canonical() == t.Addr {
			return true
		}
	}
	return false
}

func loadTagsFromFile(path string, db int, defaultDB *int) ([]schema.Tag, error) {
	data, err := readBytes(path)
	if err != nil {
		return nil, err
	}
	type row struct {
		Addr string `yaml:"addr"`
		Type string `yaml:"type"`
		Name string `yaml:"name"`
		Desc string `yaml:"desc"`
		Init any    `yaml:"init"`
	}
	var list []row
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		out := make([]schema.Tag, 0, len(list))
		for _, r := range list {
			addrValue, err := address.Parse(r.Addr, valueOrDB(defaultDB, db))
			if err != nil {
				return nil, err
			}
			out = append(out, schema.Tag{
				Addr: addrValue.Canonical(),
				Type: strings.ToUpper(strings.TrimSpace(r.Type)),
				Name: r.Name,
				Desc: r.Desc,
				Init: r.Init,
			})
		}
		return out, nil
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	records := make([][]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		records = append(records, splitCSVLine(ln))
	}
	if len(records) == 0 {
		return nil, usagef("no tags found in %s", path)
	}
	start := 0
	if looksLikeHeader(records[0]) {
		start = 1
	}
	out := make([]schema.Tag, 0, len(records)-start)
	for _, rec := range records[start:] {
		if len(rec) == 0 {
			continue
		}
		addrValue, err := address.Parse(rec[0], valueOrDB(defaultDB, db))
		if err != nil {
			return nil, err
		}
		tag := schema.Tag{Addr: addrValue.Canonical()}
		if len(rec) > 1 {
			tag.Type = strings.ToUpper(strings.TrimSpace(rec[1]))
		}
		if len(rec) > 2 {
			tag.Name = rec[2]
		}
		if len(rec) > 3 {
			tag.Desc = rec[3]
		}
		if len(rec) > 4 {
			tag.Init = rec[4]
		}
		out = append(out, tag)
	}
	return out, nil
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func looksLikeHeader(cols []string) bool {
	if len(cols) == 0 {
		return false
	}
	first := strings.ToLower(strings.TrimSpace(cols[0]))
	return first == "addr" || first == "address"
}

func heartbeatInterval(t schema.Tag) string {
	if t.Heartbeat == nil {
		return ""
	}
	return t.Heartbeat.Interval
}

func heartbeatTimeout(t schema.Tag) string {
	if t.Heartbeat == nil {
		return ""
	}
	return t.Heartbeat.Timeout
}

func setFromCSV(in string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, p := range splitCSV(in) {
		out[p] = struct{}{}
	}
	return out
}

func loadSeed(img *sim.Image, path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	for k, v := range payload {
		if err := img.Set(k, v); err != nil {
			return fmt.Errorf("seed %s: %w", k, err)
		}
	}
	return nil
}

func filterPushes(changes []sim.Change, allow map[string]struct{}, s schema.Schema, takeHB bool, enable bool, writeAll bool) ([]sim.Change, []string) {
	if !enable {
		return nil, nil
	}
	roles := map[string]string{}
	for _, t := range s.Tags {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		roles[t.Name] = strings.ToLower(strings.TrimSpace(t.Role))
	}
	out := make([]sim.Change, 0, len(changes))
	skippedHB := make([]string, 0)
	warnedHB := map[string]struct{}{}
	for _, c := range changes {
		if !writeAll {
			if _, ok := allow[c.Name]; !ok {
				continue
			}
		}
		if roles[c.Name] == "heartbeat" && !takeHB {
			if _, seen := warnedHB[c.Name]; !seen {
				warnedHB[c.Name] = struct{}{}
				skippedHB = append(skippedHB, c.Name)
			}
			continue
		}
		out = append(out, c)
	}
	return out, skippedHB
}

func printTrace(w io.Writer, format string, cycle int, changes []sim.Change, watch map[string]struct{}) {
	filtered := changes[:0]
	for _, c := range changes {
		if len(watch) == 0 {
			filtered = append(filtered, c)
			continue
		}
		if _, ok := watch[c.Name]; ok {
			filtered = append(filtered, c)
		}
	}
	if format == "json" {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cycle":   cycle,
			"changes": filtered,
		})
		return
	}
	if len(filtered) == 0 {
		return
	}
	for _, c := range filtered {
		fmt.Fprintf(w, "cycle=%d %s=%v\n", cycle, c.Name, c.Value)
	}
}
