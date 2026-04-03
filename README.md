# NSZ

A compression/decompression tool (CLI and optional desktop GUI) for certain proprietary **PFS0-style** archives and related compressed payloads, using [zstd](https://github.com/facebook/zstd). Output can be used with installers and tools that understand these formats.

**This repository:** [github.com/zyzto/nsz](https://github.com/zyzto/nsz) — Go implementation (`cmd/nsz`, `cmd/nsz-gui`, shared `internal/`). Module path: `github.com/zyzto/nsz`.

## Upstream

NSZ was created by **Nico Bosshard** with **Blake Warner** and many contributors. Upstream home:

- GitHub: [github.com/nicoboss/nsz](https://github.com/nicoboss/nsz)
- Swiss mirror: [gitlab.nicobosshard.ch/nicoboss/nsz](https://gitlab.nicobosshard.ch/nicoboss/nsz)

The [LICENSE](LICENSE) file retains the original MIT copyright and permission text required by that license.

## Legal

- This project does NOT incorporate any copyrighted material such as cryptographic keys. All keys must be provided by the user.
- This project does NOT circumvent any technological protection measures. The NSZ-related formats keep such measures in place in the usual way.
- Use this software only with data you are legally entitled to process.
- This project is MIT licensed. See [LICENSE](LICENSE) in this repository.

## Requirements

- Go 1.22+
- For **`nsz-gui`**: OS packages for Fyne/OpenGL (e.g. on Debian/Ubuntu: `libgl1-mesa-dev`, `xorg-dev`)

## Build

```bash
go build -o nsz ./cmd/nsz
go build -o nsz-gui ./cmd/nsz-gui
```

For a static, pure-Go CLI (no GUI toolkit), build only `cmd/nsz` with `CGO_ENABLED=0`.

## Keys file

Some workflows need a keys file in the format common to open-source console crypto tools (`prod.keys` or `keys.txt`), which you must obtain legally. By default the program searches the executable’s directory and your user profile using the same layout as other tools in that ecosystem (see `DefaultKeySearchPaths` in `internal/keys/keys.go` for the exact list).

The Go decompress path for standard `.nsz`/`.ncz` handled here does not require keys for those formats; compression and other features may.

## Usage

Run `nsz --help` for flags. Examples:

- Decompress a folder: `nsz -D /path/to/files/`
- Compress (when implemented): `nsz -C /path/to/files/`

`nsz-gui` provides a graphical workflow for decompression, output folder, and related options.

## Status (vs upstream)

**Implemented (Go):**

- Decompress `.nsz` → `.nsp` (PFS0 rebuild, `.ncz` → `.nca`)
- Decompress standalone `.ncz` → `.nca`
- Solid and block NCZ payloads
- CLI flags aligned with upstream where ported (some operations still return “not implemented”)
- GUI: decompress workflow, output folder, “Fix PFS0 padding”, persisted preferences

**Not implemented yet:**

- Compress (`-C`), `.xcz`/`.xci` parity, full parity for verify/titlekeys/extract/info/undupe/create, etc.

**PFS0 names:** Parsing follows upstream’s TOC walk (last row → first) when slicing the string table; see `internal/pfs0/pfs0_test.go`. The parser rejects unreasonable `fileCount`/string table sizes and out-of-range `nameOffset` values.

**NCZ implementation:** Block and section handling lives under `internal/ncz/`.

## File format details

### NSZ / PFS0 layout

`.nsz` archives follow the same layout as `.nsp` (PFS0); the extension signals compressed `.ncz` members. `.ncz` entries can be mixed with plain `.nca` members in the same container.

### XCZ

`.xcz` follows the same idea for the `.xci` style layout; the extension signals compressed `.ncz` members.

### NCZ

These are compressed `.nca`-style blobs: payload is decrypted, then compressed with zstd. The first `0x4000` bytes match the original header region (still encrypted). At `0x4000` begins the variable-sized NCZ header (sections for re-encryption metadata; optional block compression). The zstd stream follows to EOF and decompresses to offset `0x4000`. For block mode, sizes in `compressedBlockSizeList` vs decompressed block size determine whether a block is stored plain or compressed.

## Tests

```bash
go test ./... -count=1
```

## References

Historical package index entry for the original tool: <https://pypi.org/project/nsz/>

## Credits

Thanks to upstream authors and everyone who contributed to the original NSZ project (see **Upstream**).
