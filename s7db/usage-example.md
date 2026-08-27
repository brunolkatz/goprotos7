# s7db usage examples

This guide shows an end-to-end workflow:

1. initialize a YAML schema
2. add addresses/tags
3. validate and build (`pack`) a `.bin`
4. inspect values from a `.bin`
5. watch live values on PLC

---

## 1) Initialize a schema file

```bash
# create a new schema at ./s7db.yml
s7db init --db 300 --name GatewayControl
```

Example result (`s7db.yml`):

```yaml
version: 1
db: 300
name: GatewayControl
optimized: false
endian: big
size: auto
tags: []
```

---

## 2) Add addresses/tags

Add single tags:

```bash
s7db add DB300.DBX0.0 -T BOOL -n OsServiceHeartbeat -d "OS heartbeat"
s7db add DB300.DBW2   -T INT  -n MachineMode        --init 0
s7db add DB300.DBD4   -T REAL -n PumpSpeedCmd       --init 1200
```

Add multiple BOOLs:

```bash
s7db add DB300.DBX2.0 DB300.DBX2.1 DB300.DBX2.2 -T BOOL
```

Set heartbeat metadata (optional, if you use `heartbeat` command):

```yaml
tags:
  - addr: DB300.DBX0.0
    name: OsServiceHeartbeat
    type: BOOL
    init: false
    role: heartbeat
    heartbeat:
      interval: 1s
      timeout: 5s
      polarity: set-true
```

---

## 3) Validate schema

```bash
s7db check
s7db check --strict
```

Show schema summary:

```bash
s7db info
```

List tags:

```bash
s7db list
s7db list -o json
s7db list -o csv --columns addr,name,type,role,hb_interval,hb_timeout,offset,size,init,desc
```

---

## 4) Create/initialize a BIN file from YAML (pack)

```bash
# default output is DB.bin
s7db pack

# custom output
s7db pack -o /tmp/DB300.bin

# fill gaps and pad final size to 16-byte boundary
s7db pack --fill 0x00 --pad 16 -o /tmp/DB300.bin
```

---

## 5) List values from a BIN file

`s7db` does not directly list a `.bin` without schema conversion.  
Use `unpack` first, then `list`.

```bash
# best-effort unpack to YAML
s7db unpack /tmp/DB300.bin -o unpacked.yml

# list unpacked tags/values
s7db -f unpacked.yml list
s7db -f unpacked.yml list -o json
```

If you already have a template schema, unpack with it:

```bash
s7db unpack --from s7db.yml /tmp/DB300.bin -o unpacked-with-types.yml
s7db -f unpacked-with-types.yml list -o json
```

---

## 6) Watch live PLC values

Watch specific vars by name/address:

```bash
s7db watch --addr 192.168.0.10 --rack 0 --slot 1 --interval 500ms --vars OsServiceHeartbeat,MachineMode
```

One-shot read:

```bash
s7db watch --addr 192.168.0.10 --once --vars DB300.DBD4
```

Only print changes:

```bash
s7db watch --addr 192.168.0.10 --diff --vars MachineMode,PumpSpeedCmd
```

JSON output:

```bash
s7db watch --addr 192.168.0.10 --json --vars OsServiceHeartbeat,MachineMode
```

Reconnect on connection loss:

```bash
s7db watch --addr 192.168.0.10 --reconnect --vars OsServiceHeartbeat
```

---

## 7) Serve HMI + WebSocket API

Run embedded SCADA HMI (default: `127.0.0.1:8080`):

```bash
s7db serve -f s7db.yml --addr 192.168.0.10 --rack 0 --slot 1 --interval 200ms
```

Run with embedded heartbeat loop from `role: heartbeat` tag:

```bash
s7db serve -f s7db.yml --addr 192.168.0.10 --heartbeat
```

Expose non-loopback in read-only mode:

```bash
s7db serve --listen 0.0.0.0:8080 --read-only --addr 192.168.0.10
```

Expose non-loopback with write token:

```bash
s7db serve --listen 0.0.0.0:8080 --token "change-me" --addr 192.168.0.10
```

---

## 8) (Optional) Heartbeat loop (OS service)

If your schema has a single `role: heartbeat` tag:

```bash
s7db heartbeat --addr 192.168.0.10 --reconnect
```

Or target explicitly:

```bash
s7db heartbeat DB300.DBX0.0 --addr 192.168.0.10 -i 1s --heartbeat-timeout 5s --mode set-true
```

---

## 9) Example using the current `s7db.yml` (DB300 GatewayControl)

Your current file already defines tags like:
- `Heartbeat` (`DB300.DBX0.0`, role=heartbeat)
- `OSServiceControl`
- `SCADAWebserverEnable`
- `WarningMessage` / `AlarmMessage` / `InfoMessage` / `PopupDescription`
- `MillSpeedCommand` / `PumpSpeedCommand`

Use it directly:

```bash
# validate current schema
s7db -f s7db.yml check

# inspect tags (role/heartbeat columns included)
s7db -f s7db.yml list

# build DB image from current YAML
s7db -f s7db.yml pack -o /tmp/DB300.bin

# inspect built image back as schema+values
s7db -f s7db.yml unpack --from s7db.yml /tmp/DB300.bin -o /tmp/DB300-unpacked.yml
s7db -f /tmp/DB300-unpacked.yml list -o json
```

Watch live values for this schema:

```bash
s7db -f s7db.yml watch --addr 192.168.0.10 --vars Heartbeat,MillSpeedCommand,PumpSpeedCommand
```

Run heartbeat based on the `Heartbeat` tag metadata in `s7db.yml`:

```bash
s7db -f s7db.yml heartbeat --addr 192.168.0.10 --reconnect
```

---

## 10) Script example (`s7sim`) with the current `s7db.yml`

Create a script file `gateway.sim`:

```text
tick 100ms

// PLC convention: OS sets heartbeat true, PLC clears false.
on Heartbeat == true do
  Heartbeat := false
end

// If OS control is off, force commands to zero.
if OSServiceControl == false then
  MillSpeedCommand := 0.0
  PumpSpeedCommand := 0.0
end
```

Compile only (parse + typecheck):

```bash
s7db -f s7db.yml compile gateway.sim
```

Run offline for 2 seconds with trace:

```bash
s7db -f s7db.yml sim gateway.sim --duration 2s --trace
```

Run against PLC in pull-only mode (default when `--plc` is set):

```bash
s7db -f s7db.yml sim gateway.sim --plc --addr 192.168.0.10 --duration 5s --trace
```

Run with controlled writes (allowlist):

```bash
s7db -f s7db.yml sim gateway.sim --plc --write --allow-write MillSpeedCommand,PumpSpeedCommand --addr 192.168.0.10
```

Run with `--write-all` (every assigned schema tag except heartbeat unless `--take-heartbeat`):

```bash
s7db -f s7db.yml sim gateway.sim --plc --write --write-all --addr 192.168.0.10 --dry-run
```
