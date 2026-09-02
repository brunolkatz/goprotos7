# s7sim language definitions

This document explains how to **read and write your own `.sim` scripts** for `s7db`.

---

## 1) What s7sim is

`s7sim` is a small IEC-ST-like script language used by:

- `s7db compile <file.sim>` (parse + typecheck)
- `s7db sim <file.sim>` (execute by tick, offline or with PLC I/O)
- `s7db lsp --stdio` (editor diagnostics using the same compile pipeline)

Default execution is **offline memory image**.  
PLC access is opt-in using `--plc`.

---

## 2) Script structure

A script has:

1. one `tick` declaration (required, first statement)
2. optional `var` declarations
3. executable statements (`if`, `on`, assignments, `pulse`, `once pulse`)

Important rule: simple statements end with `;`:

- `tick 100ms;`
- `var foo : BOOL := true;`
- `Foo := false;`
- `pulse Foo;`
- `once pulse Foo, 2;`

Compound headers do not take `;` before `then`/`do`/`else`; `end;` is optional.

Example skeleton:

```text
tick 100ms;
var my_timer : TIME := T#0s;

if SomeBool then
  SomeValue := 10;
end;
```

---

## 3) Keywords

- `tick`
- `var`
- `once`
- `pulse`
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
var fault_timer : TIME := T#0s;
var retries : INT := 0;
var title : STRING[20] := "idle";
```

Variables and schema tag names share one namespace (avoid duplicates).

Built-in:

- `tick` (type `TIME`) = current cycle period

---

## 7) Statements

### Assignment

```text
MillSpeedCommand := 1200.0;
OsServiceFault := true;
fault_timer := fault_timer + tick;
```

### Pulse

`pulse` creates a momentary BOOL pulse, useful for one-shot button-like events.

```text
pulse PopupOKActivationPulse;
pulse PopupOKActivationPulse, 2;
```

- `pulse NAME;` is equivalent to `pulse NAME, 2;`
- `N` is a tick count, not a duration like `T#200ms`
- The tag is set to `true` immediately and stays true for `N` ticks total, then clears to `false`
- The pulse is non-retriggerable while already active
- Heartbeat tags cannot be pulsed

### Once pulse

`once pulse` is a statement-latched pulse.

```text
once pulse PopupOKActivationPulse;
once pulse PopupOKActivationPulse, 2;
```

- `once pulse` only applies to pulse statements (not assignments)
- Default width is still `2` ticks
- It fires once while the statement keeps executing, then stays quiet
- It re-arms when the statement is skipped in a cycle
- Site identity is per statement location, so two `once pulse Lamp` lines latch independently

Example:

```text
tick 100ms;
on rising OSServiceControl do
  pulse PopupOKActivationPulse;
end;
```

### If / Else

```text
if Level > 90.0 then
  PumpEnable := false;
else
  PumpEnable := true;
end;
```

### On trigger

Level-triggered each cycle:

```text
on Heartbeat == true do
  Heartbeat := false;
end
```

Edge-triggered:

```text
on rising StartCmd do
  PumpEnable := true;
end

on falling StartCmd do
  PumpEnable := false;
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
  PumpEnable := false;
end
```

---

## 9) Execution model

- One cycle per `tick`
- Script runs top-to-bottom every cycle
- `s7db sim` can stop by `--duration`, `--cycles`, or Ctrl+C
- Simple statements must end with `;` (`tick`, `var`, assignments, `pulse`, `once pulse`)
- `if ... then`, `on ... do`, `else` headers do not take `;` (`end;` is allowed)
- `pulse` is processed after the main statement list in the current cycle and decrements on the next tick(s)
- `once pulse` disarms after execution and re-arms after any cycle where that statement was skipped

Offline mode:

- uses the local schema and memory image
- supports `--plc` for live writes when allowed
- variables share the same namespace as schema tags

---

## 10) Diagnostics and compile behavior

The compiler reports errors with a file, line, column, snippet, and caret. Examples:

- missing `;` after a statement
- unterminated strings
- unknown names and type mismatches
- `pulse` width must be an integer literal `>= 1`
- `"once" can only prefix pulse`
- heartbeat tags cannot be pulsed

Compiler output is designed to point at the actual token that caused the error, not at the start of the file.

---

## 11) Examples

```text
tick 100ms;
var fault_timer : TIME := T#0s;

if OSServiceControl == true then
  AlarmMessage := "Meu alarme";
end;

if OSServiceControl == false then
  AlarmMessage := "";
end;

on Foo == true do
  Bar := false;
end;
```

```text
tick 100ms;
on rising OSServiceControl do
  pulse PopupOKActivationPulse;
end;

if SomeBit == true then
  pulse PopupOKActivationPulse, 2;
end;
```

```text
tick 100ms;
if OSServiceControl == true then
  AlarmMessage := "Meu alarme";
  once pulse PopupOKActivationPulse, 2;
end;
if OSServiceControl == false then
  AlarmMessage := "";
end;
```

---

## 12) Notes for editors

- TextMate highlighting is available in `editors/s7sim/`
- `s7db lsp --stdio -f ${workspace}/.s7db/s7db.yml` can publish compile diagnostics in the editor
- The grammar highlights comments, strings, keywords, numeric/time literals, `STRING[n]`, semicolons, `pulse`, and `once`

This file is a practical reference for writing scripts that compile cleanly with `s7db compile`.
