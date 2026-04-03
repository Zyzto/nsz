# NSZ (Go)

> **Experimental:** This Go port is incomplete compared to upstream NSZ. Several formats and flags are missing or only partially tested. **It may not work** on your files, produce bit-identical output, or match Python NSZ behavior. Prefer [upstream NSZ](https://github.com/nicoboss/nsz) for anything important until you have verified this build yourself.

A **Go** implementation of the NSZ family of tools: compress and decompress Nintendo Switch–style **PFS0** archives (`.nsp` / `.nsz`) and **NCZ** payloads (compressed `.nca`), using [zstd](https://github.com/facebook/zstd). This repo ships two programs that share the same core library:

| Binary      | Role |
|------------|------|
| **`nsz`**  | CLI only (`CGO_ENABLED=0` supported); no GUI dependencies |
| **`nsz-gui`** | Desktop UI ([Fyne](https://fyne.io/)) on top of the same `internal/core` API |

**Repository:** [github.com/zyzto/nsz](https://github.com/zyzto/nsz) · **Go module:** `github.com/zyzto/nsz`

---

## Upstream

NSZ was created by **Nico Bosshard** with **Blake Warner** and many contributors. This port tracks behavior and flags where practical but is **not** a line-for-line clone.

| Resource | URL |
|----------|-----|
| Upstream GitHub | [github.com/nicoboss/nsz](https://github.com/nicoboss/nsz) |
| Swiss mirror | [gitlab.nicobosshard.ch/nicoboss/nsz](https://gitlab.nicobosshard.ch/nicoboss/nsz) |
| PyPI (original) | [pypi.org/project/nsz/](https://pypi.org/project/nsz/) |

The [LICENSE](LICENSE) file retains the MIT text from upstream as required.

---

## Legal

- This project does **not** ship cryptographic keys. You must supply any keys file yourself, from lawful sources.
- This project does **not** exist to circumvent protection measures; formats handled here preserve the usual encryption layout.
- Use the software only on data you are entitled to process.
- Licensed under MIT — see [LICENSE](LICENSE).

---

## Requirements

- **Go 1.22+**
- **GUI build (`nsz-gui`):** system packages for OpenGL / Fyne (e.g. Debian/Ubuntu: `libgl1-mesa-dev`, `xorg-dev`)

CI installs those packages on Ubuntu before `go test` and builds both binaries (see [.github/workflows/go.yml](.github/workflows/go.yml)).

---

## Build

```bash
go build -o nsz ./cmd/nsz
go build -o nsz-gui ./cmd/nsz-gui
```

For a **static, pure-Go CLI** (no OpenGL), build only the CLI:

```bash
CGO_ENABLED=0 go build -o nsz ./cmd/nsz
```

---

## Quick start (CLI)

The commands below may fail or differ from Python NSZ; treat them as **best-effort** until you have validated them on your data.

```bash
# Help (all flags mirror upstream names where ported)
nsz --help

# Decompress .nsz → .nsp (output next to input if -o omitted)
nsz -D game.nsz

# Decompress into a directory
nsz -D -o /path/out game.nsz

# Standalone .ncz → .nca
nsz -D patch.ncz

# Compress a single .nca → .ncz (solid zstd; see limitations below)
nsz -C -l 18 title.nca

# Info (PFS0 / NCZ / .xci root layout)
nsz -i -depth 1 archive.nsp

# Verify NCZ members inside .nsz / .nsp / standalone .ncz
nsz -V -Q bundle.nsz

# Title keys from tickets in .nsp/.nsz → merges ./titlekeys.txt
nsz -titlekeys game.nsp
```

**Machine-readable progress** (for scripts):

```bash
nsz -D --machine-readable game.nsz
```

---

## GUI (`nsz-gui`)

- Same **experimental** expectations as the CLI: verify outputs before trusting them.
- Queue paths, optional **output directory**, **Compress** / **Decompress**, **Verify**, **Info**, **Extract** (extract errors until implemented), **Dump keys list** (titlekeys).
- **Settings:** zstd level, block vs solid preference labels, block size exponent, verify mode (off / quick / full), keep-all, long mode, threads, multi-job count, overwrite, fix padding, parse CNMT flags, **compress .xci → .xcz** (off by default).
- **Theme:** custom dark styling; preferences persist under the Fyne app ID `io.github.zyzto.nsz.gui`.
- When `core.Run` fails, the status line shows `Error: …` and an error dialog is shown.
- Jobs also log to **stderr** with timestamps (useful when launched from a terminal).

---

## Keys file (`prod.keys` / `keys.txt`)

Some **future** or **upstream-equivalent** workflows expect a keys file in the usual ecosystem format. **Decompressing** standard `.nsz` / `.ncz` in this port does **not** require keys.

Search paths are defined in `internal/keys/keys.go` (`DefaultKeySearchPaths`): executable directory, common profile locations, etc.

---

## What works today (summary)

| Area | Status |
|------|--------|
| Decompress `.nsz` → `.nsp` | Yes (PFS0 rebuild, `.ncz` members → `.nca`) |
| Decompress `.ncz` → `.nca` | Yes |
| Decompress `.xcz` | **No** (fails if that is all you pass) |
| Compress `.nca` → `.ncz` | Yes (**solid** only; block mode warns) |
| Compress `.xci` → `.xcz` | Optional (`-compress-xci` / GUI setting); experimental |
| Compress whole `.nsp` | **No** (returns structured error; needs full container pipeline) |
| Info | `.nsp` / `.nsz`, `.ncz`, `.xci` (depth ≥2 secure listing limited) |
| Verify | NCZ payload checks; not full CNMT / plain-NCA hash parity |
| Titlekeys | `.nsp` / `.nsz` → `titlekeys.txt`; no `titledb/` JSON merge |
| Extract (`-x`) | **No** |
| Undupe / create / other flags | Parsed but not wired in `core.Run` |

A longer parity and limitation list lives in [docs/PARITY.md](docs/PARITY.md).

---

## File formats (short)

- **`.nsp` / `.nsz`:** PFS0 container. `.nsz` is the same layout with some entries stored as `.ncz` instead of raw `.nca`.
- **`.ncz`:** Compressed NCA-style blob: first `0x4000` bytes match the original encrypted header window; NCZ metadata and zstd stream follow (solid or block layout — this port implements read/write for the pieces used in tests and CLI paths above).
- **`.xci` / `.xcz`:** Cartridge-style layout; `.xcz` is the compressed analogue. **Decompress / info / verify for `.xcz`** are largely unimplemented; **compress** to `.xcz` is opt-in.

---

## Tests

```bash
go test ./... -count=1
```

`internal/ncz` and other packages include golden-style tests against expectations aligned with the original Python implementation.

---

## Project layout

```
cmd/nsz/          CLI entrypoint
cmd/nsz-gui/      Fyne GUI
internal/core/    Options, Run(), compress/decompress/info/verify/titlekeys
internal/ncz/     NCZ encode/decode, block/solid
internal/pfs0/    PFS0 reader
internal/hfs0/    HFS0 for .xci / secure partition views
internal/xci/     XCI layout helpers
internal/ticket/  Ticket parsing for titlekeys
internal/keys/    Key file loading search paths and helpers
```

---

## Credits

Thanks to upstream authors and everyone who contributed to the original NSZ project. This Go port is maintained separately; see **Upstream** for the canonical feature set and documentation of the Python tool.
