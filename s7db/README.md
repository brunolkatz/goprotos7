# s7db

`s7db` is a Go CLI for managing Siemens S7 Data Block schemas and producing raw DB images.

## Install / Build

```bash
go build -o s7db ./cmd/s7db
```

## Schema format

Default schema path is `./s7db.yml`.

```yaml
version: 1
db: 10
name: ProcessPara
optimized: false
endian: big
size: auto
tags:
  - addr: DB10.DBX0.0
    name: ValveOpen
    type: BOOL
    init: false
    desc: Discharge valve open feedback
  - addr: DB10.DBW2
    name: Setpoint
    type: INT
    init: 250
    desc: Speed setpoint 0-1000
  - addr: DB10.DBD4
    name: Level
    type: REAL
    init: 12.5
    desc: Tank level in liters
```

Supported types: `BOOL`, `BYTE`, `CHAR`, `SINT`, `USINT`, `INT`, `UINT`, `WORD`, `DINT`, `UDINT`, `DWORD`, `REAL`, `TIME`, `STRING[n]`.

Supported address forms:
- full: `DB10.DBX0.0`, `DB10.DBB1`, `DB10.DBW2`, `DB10.DBD4`
- short (with `--db`): `X0.0`, `B1`, `W2`, `D4`

## Global flags

```bash
s7db [global flags] <command> [flags] [args]
```

Common global flags:
- `-f, --file` schema path (default `./s7db.yml`)
- `-c, --config` config path (`~/.config/s7db/config.yaml` or `$S7DB_CONFIG`)
- `-t, --timeout` timeout duration (default `10s`)
- `--db`, `--endian`, `--strict`, `--dry-run`, `--force`, `--no-color`, `-q`, `-v`
- `--version`

## Commands

### init
Create a new schema file.

```bash
s7db init --db 10 --name Demo
s7db init --from existing.yml --force
s7db init --from existing.yml --merge --force
```

### reset
Wipe and optionally re-import schema (same flags as `init`).

```bash
s7db reset --from existing.yml --force
```

### add
Add one or more tags.

```bash
s7db add DB10.DBW2 --type INT --name Setpoint --desc "Speed SP" --init 250
s7db add DB10.DBX0.0 DB10.DBX0.1 --type BOOL
s7db add --from tags.csv
s7db add --from tags.yaml
```

CSV columns (header optional): `addr,type,name,desc,init`.

### set
Update an existing tag by address or name.

```bash
s7db set DB10.DBW2 --desc "new" --init 300
s7db set Setpoint --name SpeedSP
```

### rm / remove
Remove one or more tags by address or name.

```bash
s7db rm DB10.DBX0.0
s7db remove Setpoint
```

### list
List schema tags.

```bash
s7db list
s7db list -o json
s7db list -o csv --columns addr,name,type,offset,size,init,desc
s7db list --addr Setpoint
```

### pack
Pack schema into a raw binary DB image.

```bash
s7db pack -o /tmp/DB10.bin
s7db pack --fill 0xFF --pad 16 -o -
```

`--format mc7` currently returns a clear not-implemented error.

### unpack
Best-effort unpack from `.bin` into schema YAML.

```bash
s7db unpack /tmp/DB10.bin > out.yml
s7db unpack --from template.yml /tmp/DB10.bin -o out.yml
```

### check
Validate layout and report overlaps/type issues.

```bash
s7db check
s7db check --strict
```

### info
Print schema summary.

```bash
s7db info
```

### watch
Live-read variables from PLC via `github.com/get-notify/gos7`.

`watch` always prints both:
- raw PLC bytes (`raw`, hex)
- decoded typed value (`value`)

Decode rules:
- tag name operand (`-f/--file` schema): uses the tag `type` and schema endian
- address operand: infers fixed types from address letter
  - `DBX` / `X` → `BOOL`
  - `DBB` / `B` → `BYTE`
  - `DBW` / `W` → `WORD` (unsigned)
  - `DBD` / `D` → `DWORD` (unsigned)
- for signed/float semantics (`INT`, `DINT`, `REAL`), use schema tags by name

```bash
s7db watch --addr 192.168.0.10 --vars Setpoint,DB10.DBX0.0
s7db watch --addr 192.168.0.10 Setpoint Level --interval 1s --diff
s7db watch --addr 192.168.0.10 --once --json
s7db watch --addr 192.168.0.10 --reconnect --count 0
```

Human output columns are:
- `ts`
- `name-or-addr`
- `type`
- `raw`
- `value`

Example:

```text
2026-08-26T14:45:01Z	OsServiceHeartbeat	BOOL	01	true
2026-08-26T14:45:01Z	DB300.DBW2	WORD	00 FA	250
2026-08-26T14:45:01Z	Level	REAL	41 48 00 00	12.5
```

JSON (`--json`) keeps `values` and adds typed per-variable records:

```json
{
  "ts": "2026-08-26T14:45:01Z",
  "values": {
    "OsServiceHeartbeat": true
  },
  "samples": [
    {
      "addr": "DB300.DBX0.0",
      "name": "OsServiceHeartbeat",
      "type": "BOOL",
      "raw": "01",
      "raw_hex": "01",
      "value": true,
      "bit": 0
    }
  ]
}
```

### serve (HMI)
Serve an embedded industrial HMI and WebSocket/REST API.

```bash
s7db serve -f machine.yml --addr 192.168.0.10 --heartbeat
# open http://127.0.0.1:8080
```

Key points:
- Static UI is embedded in the binary (`internal/server/static`) with no npm/build step.
- Existing `watch`, `heartbeat`, and `pack` commands remain unchanged in usage.
- Default bind is loopback (`127.0.0.1:8080`).
- For non-loopback writes, provide `--token` (or run `--read-only`, or `--force`).

YAML UI examples:

```yaml
tags:
  - addr: DB300.DBX0.0
    name: OsServiceHeartbeat
    type: BOOL
    role: heartbeat
    heartbeat: { interval: 1s, timeout: 5s, polarity: set-true }
    ui: { widget: status, group: System }
  - addr: DB300.DBX0.1
    name: ValveOpen
    type: BOOL
    ui: { widget: button, mode: toggle, group: Actuators }
  - addr: DB300.DBW2
    name: Setpoint
    type: INT
    ui: { widget: knob, min: 0, max: 1000, step: 1, unit: rpm, group: Process }
```

WebSocket (`/ws`) operations:
- client → server: `sub`, `set`, `ping`
- server → client: `snap`, `upd`, `ack`, `hb`, `pong`

Client reconnect is handled in `app.js` with exponential backoff (500ms..5s).

### heartbeat (OS Service)
Write the heartbeat watchdog bit from OS to PLC.

Schema example:

```yaml
tags:
  - addr: DB300.DBX0.0
    name: OsServiceHeartbeat
    type: BOOL
    init: false
    desc: Heartbeat OS Service moves to true PLC Moves to false
    role: heartbeat
    heartbeat:
      interval: 1s
      timeout: 5s
      polarity: set-true
```

CLI examples:

```bash
s7db heartbeat -f machine.yml --reconnect
s7db heartbeat DB300.DBX0.0 -i 1s --heartbeat-timeout 5s --addr 192.168.0.10 --rack 0 --slot 1
```

Modes:
- `set-true`: OS writes `true` each beat, PLC clears to `false`
- `toggle`: OS alternates `0`/`1`

Notes:
- PLC fault handling and safe-state actions remain PLC logic; `s7db` only refreshes the bit and reports communication health.
- On shutdown, `s7db` does **not** force-write `false`; PLC owns the clear behavior.

### Simulation language (s7sim)

Compile script:

```bash
s7db compile machine.sim
```

Run offline (default, no PLC I/O):

```bash
s7db sim machine.sim --duration 10s --trace
```

Run with PLC pull-only (`--plc` defaults to read-only):

```bash
s7db sim machine.sim --plc --addr 192.168.0.10 --rack 0 --slot 1
```

Run with controlled write allowlist (dry-run):

```bash
s7db sim machine.sim --plc --write --allow-write OsServiceFault --dry-run --addr 192.168.0.10
```

Allow heartbeat writes explicitly:

```bash
s7db sim machine.sim --plc --write --allow-write OsServiceHeartbeat --take-heartbeat --addr 192.168.0.10
```

Example script:

```text
tick 100ms
var fault_timer : TIME := T#0s
on OsServiceHeartbeat == true do
  OsServiceHeartbeat := false
end
if OsServiceHeartbeat == false then
  fault_timer := fault_timer + tick
else
  fault_timer := T#0s
end
if fault_timer > T#5s then
  OsServiceFault := true
else
  OsServiceFault := false
end
```

Notes:
- This is an external tick task, not PLC CPU logic.
- `--plc --write` requires `--allow-write`.
- Heartbeat tags are not pushed unless `--take-heartbeat`.

## Config file

Path: `~/.config/s7db/config.yaml` (or `$S7DB_CONFIG`).x

```yaml
file: ./s7db.yml
db: 10
endian: big
timeout: 10s
plc:
  addr: 192.168.0.10
  rack: 0
  slot: 1
  port: 102
```

## Exit codes

- `0`: success
- `1`: runtime / PLC / I/O error
- `2`: usage / validation error

Stdout is data-only output; diagnostics are written to stderr.
