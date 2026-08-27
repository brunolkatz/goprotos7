# s7sim syntax highlighting (JetBrains TextMate bundle)

This folder provides **TextMate-based highlighting** for `*.sim` files used by `s7db sim`.

- Scope: `source.s7sim`
- File extension: `.sim`
- Highlighting only (no parser, no IDE diagnostics)

`s7db compile` remains the source of syntax/type/runtime diagnostics.

## Install in GoLand / IntelliJ

1. Open **Settings** (or **Preferences** on macOS).
2. Go to **Editor → TextMate Bundles**.
3. Click **+** and select this folder:
   - `/Users/brunokatzjarowski/sistemas/personal/goprotos7/s7db/editors/s7sim`
4. Open (or reopen) a `*.sim` file.

If IntelliJ shows **“Cannot read bundle”**, remove the failed entry and add the same folder again (this bundle now includes `package.json`, which JetBrains expects in many setups).

If `.sim` is mapped to another file type:

1. Go to **Settings → Editor → File Types**.
2. Find the conflicting type and remove `*.sim` from it.
3. Ensure `*.sim` is handled by the TextMate bundle.

## What should highlight

In `examples/mill.sim`, you should see:

- `tick`, `if`, `then`, `else`, `end`, `on`, `do` as keywords
- `T#100ms`, `T#5s` as time literals
- `STRING[20]`, `BOOL`, `REAL`, `TIME` as types
- `"os fault"` as a quoted string
- `//` comments dimmed
- `:=`, `==`, `>`, `+`, `(`, `)` as operators

## Notes

- This does not provide red squiggles or semantic validation.
- Use `s7db compile your-script.sim` to validate scripts.
