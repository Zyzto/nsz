# Upstream parity and limitations (Go port)

**This port is experimental.** Behavior can differ from Python NSZ; edge cases may fail or corrupt output. Do not assume parity—validate outputs (hashes, installs) before relying on this tool.

This document compares **this repository** (`github.com/zyzto/nsz`) to **upstream NSZ** ([nicoboss/nsz](https://github.com/nicoboss/nsz)). The CLI accepts many of the same flags for compatibility; not every flag has an effect yet.

## Implemented in Go

### Decompress (`-D`)

- **`.nsz` → `.nsp`:** Rebuilds PFS0; rewrites `.ncz` members to `.nca`.
- **`.ncz` → `.nca`:** Standalone NCZ files.
- **`-F` / `-fix-padding`:** Passed through to the NSZ decompress path where applicable.
- **`-o` / output directory:** Must exist when set; otherwise error.

### Compress (`-C`)

- **`.nca` → `.ncz`:** Solid zstd compression via `internal/ncz` (level, long mode, thread count for encoding).
- **`.xci` → `.xcz`:** Only when **explicitly enabled** (`-compress-xci` or GUI “compress XCI”). Off by default; treated as experimental.

### Info (`-i`)

- **`.nsp` / `.nsz`:** PFS0 listing (`-depth` controls nesting).
- **`.ncz`:** NCZ header / section summary.
- **`.xci`:** Root HFS0-style layout (secure partition deep listing may warn for `depth >= 2`).

### Verify (`-V`, `-Q`)

- **`.ncz`:** Decompress + hash check of payload.
- **`.nsp` / `.nsz`:** Same for each `.ncz` member.
- **“Full” vs “quick”:** Full verify still does **not** rebuild entire PFS0/XCI and compare CNMT-listed hashes for every plain `.nca`; a warning is printed and NCZ-focused checks run.

### Titlekeys (`-titlekeys`)

- Reads **`.nsp` / `.nsz`**, finds `.tik`, parses ticket, merges into **`titlekeys.txt`** in the current working directory (load existing file first).
- If the input queue contains paths but **none** are `.nsp`/`.nsz`, `Run` returns an error (avoids a silent no-op).
- **`titledb/` JSON merge** (Python updates per-title JSON) is **not** implemented; a warning is emitted if a `titledb` directory exists.

### CLI quality-of-life

- **`-machine-readable`:** JSON lines for progress on stdout.
- **Progress bar** (non-quiet) via `progressbar`.
- Exit code **3** for certain “container compress not implemented” errors (`ErrCompressContainerNotImplemented`).

### GUI

- Shared `core.Run` with reporter channels; settings persisted; error dialog on failure.

---

## Not implemented or partial

| Feature | Notes |
|---------|--------|
| **Decompress `.xcz`** | Skipped with warning; **error** if no `.nsz`/`.ncz` was decompressed in the same run. |
| **Info / verify `.xcz`** | Warning per file; **error** if no other supported file produced work in that mode. |
| **Verify `.xci`** | Same pattern as `.xcz` for verify-only queues. |
| **Compress `.nsp`** | Returns `ErrCompressContainerNotImplemented` — needs full PFS0 + per-NCA pipeline like upstream. |
| **Compress block mode (`-B`)** | Warns; solid path only for `.nca`. |
| **Post-compress verify (`-V` with `-C`)** | Warns; not run. |
| **Extract (`-x`)** | Returns error from `Run`. |
| **Undupe** (`-undupe` and related flags) | Flags exist on CLI; not handled in `core.Run`. |
| **Create (`-c`)** | Not handled in `core.Run`. |
| **CNMT parse flags (`-p`, `-P`)** | Present on options; wiring for compress workflows may be incomplete vs Python. |
| **Full CNMT hash verification** | NCZ decompress+SHA256 only; not full ROM/package hash matrix. |

---

## Behavioral notes

- **`.xci` compress** defaults to **skipped** unless `-compress-xci` is set (matches GUI default).
- **Wrong-input jobs:** Operations that would do no useful work (e.g. only `.xcz` for info/verify/decompress, or titlekeys with only non-NSP files) return **errors** so scripts and the GUI do not report false success.

---

## PFS0 and NCZ details

- **PFS0:** Name table iteration follows upstream’s TOC order (see `internal/pfs0` tests).
- **NCZ:** Block and solid readers/writers live under `internal/ncz/`; tests document golden expectations.

For low-level layout, the main [README](../README.md) has a short format section; upstream docs and source remain the reference for every edge case.
