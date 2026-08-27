# s7sim language definitions

This document explains how to **read and write your own `.sim` scripts** for `s7db`.

---

## 1) What s7sim is

`s7sim` is a small IEC-ST-like script language used by:

- `s7db compile <file.sim>` (parse + typecheck)
- `s7db sim <file.sim>` (execute by tick, offline or with PLC I/O)

Default execution is **offline memory image**.  
PLC access is opt-in using `--plc`.

---

## 2) Script structure

A script has:

1. one `tick` declaration (required, first statement)
2. optional `var` declarations
3. executable statements (`if`, `on`, assignments)

Example skeleton:

```text
tick 100ms
var my_timer : TIME := T#0s

if SomeBool then
  SomeValue := 10
end
```

---

## 3) Keywords

- `tick`
- `var`
- `if`, `then`, `else`, `end`
- `on`, `do`
- `and`, `or`
- `true`, `false`
- `rising`, `falling`

Comments use `//`:

```text
// this is a comment
```

---

## 4) Literals

### Boolean

- `true`
- `false`

### Numbers

- Integer: `10`, `0`, `-5`
- Real: `12.5`, `0.0`, `-3.14`

### Time

- `100ms`
- `T#5s`
- `T#1m`
- `T#2h`

---

## 5) Types

Supported variable/tag types for script typing:

- `BOOL`
- `INT` / integer-family tags (`INT`, `DINT`, `UINT`, `WORD`, `DWORD`)
- `REAL`
- `TIME`
- `STRING[n]` (n must be `1..254`)

---

## 6) Variables

Declare local variables with `var`:

```text
var fault_timer : TIME := T#0s
var retries : INT := 0
var title : STRING[20] := "idle"
```

Variables and schema tag names share one namespace (avoid duplicates).

Built-in:

- `tick` (type `TIME`) = current cycle period

---

## 7) Statements

### Assignment

```text
MillSpeedCommand := 1200.0
OsServiceFault := true
fault_timer := fault_timer + tick
```

### If / Else

```text
if Level > 90.0 then
  PumpEnable := false
else
  PumpEnable := true
end
```

### On trigger

Level-triggered each cycle:

```text
on Heartbeat == true do
  Heartbeat := false
end
```

Edge-triggered:

```text
on rising StartCmd do
  PumpEnable := true
end

on falling StartCmd do
  PumpEnable := false
end
```

---

## 8) Expressions and operators

### Logical

- `and`
- `or`

### Comparison

- `==`, `!=`
- `>`, `>=`, `<`, `<=`

### Arithmetic

- `+`, `-`, `*`, `/`

For `STRING`, only `==` and `!=` are supported in v1.
`+` concatenation and ordering (`<`, `>`, etc.) are compile errors.

### Parentheses

```text
if (ValveOpen and Level > 90.0) or ForceStop then
  PumpEnable := false
end
```

---

## 9) Execution model

- One cycle per `tick`
- Script runs top-to-bottom every cycle
- `s7db sim` can stop by `--duration`, `--cycles`, or Ctrl+C

Offline mode:

- reads/writes happen only in memory image initialized from schema `init`

PLC mode (`--plc`):

1. pull referenced values from PLC
2. execute one cycle
3. push allowed changed values (if write enabled)

---

## 10) Type rules (practical)

- `if` / `on` conditions must evaluate to `BOOL`
- Assignments must match target type
- Numeric mix is allowed where sensible (`INT` with `REAL` becomes real math)
- `TIME` math supports `+` / `-` with `TIME`
- `STRING[n]` assignment accepts literals and other strings; values longer than `n` are truncated
- Division by zero is runtime error (simulation stops)

---

## 11) Common errors (and fixes)

### Unknown name

```text
unknown name "OsServiceHeartbat"
hint: did you mean OsServiceHeartbeat?
```

Fix: use exact schema tag name.

### Type mismatch

```text
cannot assign REAL to BOOL
```

Fix: write a BOOL expression/constant (`true`/`false`) to BOOL targets.

### Invalid STRING declaration

```text
type error: STRING requires a max length
hint: use STRING[20] (1..254)
```

Fix: declare `STRING[n]` explicitly.

### STRING concatenation

```text
type error: string concatenation not supported
hint: assign a full literal
```

---

## 12) End-to-end examples

### Example A — Heartbeat + fault timer

```text
tick 100ms
var fault_timer : TIME := T#0s

on Heartbeat == true do
  Heartbeat := false
end

if Heartbeat == false then
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

### Example B — Process interlock

```text
tick 200ms

if ValveOpen and Level > 90.0 then
  PumpEnable := false
end

if not_used == false then
  // placeholder example for additional logic
end
```

(Use valid schema names in your real script.)

---

## 13) CLI usage

Compile:

```bash
s7db -f s7db.yml compile machine.sim
```

Run offline:

```bash
s7db -f s7db.yml sim machine.sim --duration 10s --trace
```

Run with PLC pull-only:

```bash
s7db -f s7db.yml sim machine.sim --plc --addr 192.168.0.10
```

Run with controlled writes:

```bash
s7db -f s7db.yml sim machine.sim --plc --write --allow-write MillSpeedCommand,PumpSpeedCommand --addr 192.168.0.10
```

Run with write-all:

```bash
s7db -f s7db.yml sim machine.sim --plc --write --write-all --addr 192.168.0.10
```

Allow heartbeat writes explicitly:

```bash
s7db -f s7db.yml sim machine.sim --plc --write --allow-write Heartbeat --take-heartbeat --addr 192.168.0.10
```
