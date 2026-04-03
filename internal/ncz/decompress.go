package ncz

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/zyzto/nsz/internal/aesutil"
)

// Decompress converts NCZ from rs into w (nil = hash only). Matches Python __decompressNcz.
func Decompress(rs io.ReadSeeker, w io.Writer, onProgress func(readPos, decompressed int64, total int64, step string)) (written int64, hexHash string, err error) {
	if _, err = rs.Seek(0, io.SeekStart); err != nil {
		return 0, "", err
	}
	header := make([]byte, UncompressibleHeaderSize)
	if _, err = io.ReadFull(rs, header); err != nil {
		return 0, "", err
	}
	magic := make([]byte, 8)
	if _, err = io.ReadFull(rs, magic); err != nil {
		return 0, "", err
	}
	if string(magic) != "NCZSECTN" {
		return 0, "", fmt.Errorf("ncz: missing NCZSECTN magic")
	}
	var sectionCount int64
	if err = binary.Read(rs, binary.LittleEndian, &sectionCount); err != nil {
		return 0, "", err
	}
	if sectionCount < 0 || sectionCount > 4096 {
		return 0, "", fmt.Errorf("ncz: invalid section count %d", sectionCount)
	}
	sections := make([]Section, sectionCount)
	for i := range sections {
		sections[i], err = ReadSection(rs)
		if err != nil {
			return 0, "", err
		}
	}
	if len(sections) > 0 && sections[0].Offset > UncompressibleHeaderSize {
		fs := Section{
			Offset:     UncompressibleHeaderSize,
			Size:       sections[0].Offset - UncompressibleHeaderSize,
			CryptoType: 1,
		}
		sections = append([]Section{fs}, sections...)
	}
	var ncaSize int64 = UncompressibleHeaderSize
	for _, s := range sections {
		if s.Offset >= UncompressibleHeaderSize {
			ncaSize += s.Size
		}
	}

	pos, err := rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, "", err
	}
	peek := make([]byte, 8)
	if _, err = io.ReadFull(rs, peek); err != nil {
		return 0, "", err
	}
	if _, err = rs.Seek(pos, io.SeekStart); err != nil {
		return 0, "", err
	}
	useBlock := string(peek) == "NCZBLOCK"

	var br *BlockReader
	var zr *zstd.Decoder
	var solid io.Reader

	if useBlock {
		bh, e := ReadBlockHeader(rs)
		if e != nil {
			return 0, "", e
		}
		var dataStart int64
		dataStart, err = rs.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, "", err
		}
		br, err = NewBlockReader(rs, bh, dataStart)
		if err != nil {
			return 0, "", err
		}
		defer br.Close()
	} else {
		zr, err = zstd.NewReader(rs)
		if err != nil {
			return 0, "", err
		}
		defer zr.Close()
		solid = zr
	}

	h := sha256.New()
	var startOut int64
	if w != nil {
		if sw, ok := w.(io.Seeker); ok {
			startOut, err = sw.Seek(0, io.SeekCurrent)
			if err != nil {
				return 0, "", err
			}
		}
	}
	if w != nil {
		if _, err = w.Write(header); err != nil {
			return 0, "", err
		}
	}
	if _, err = h.Write(header); err != nil {
		return 0, "", err
	}
	decompressed := int64(len(header))
	readLogical := int64(len(header))

	if onProgress != nil {
		onProgress(readLogical, decompressed, ncaSize, "Decompress")
	}

	firstSection := true
	buf := make([]byte, 0x10000)

	for _, s := range sections {
		i := s.Offset
		end := s.Offset + s.Size
		if firstSection {
			firstSection = false
			uncompressedSize := UncompressibleHeaderSize - sections[0].Offset
			if uncompressedSize > 0 {
				i += uncompressedSize
			}
		}
		useCrypto := s.CryptoType == 3 || s.CryptoType == 4

		for i < end {
			chunkSz := int64(0x10000)
			if end-i < chunkSz {
				chunkSz = end - i
			}
			if int(chunkSz) > len(buf) {
				buf = make([]byte, chunkSz)
			}

			var chunk []byte
			if useBlock {
				logical := i - UncompressibleHeaderSize
				if _, err = br.Seek(logical, io.SeekStart); err != nil {
					return 0, "", err
				}
				n, rerr := io.ReadFull(br, buf[:chunkSz])
				if rerr != nil && rerr != io.ErrUnexpectedEOF {
					return 0, "", rerr
				}
				chunk = buf[:n]
			} else {
				n, rerr := io.ReadFull(solid, buf[:chunkSz])
				if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
					return 0, "", rerr
				}
				if n == 0 {
					break
				}
				chunk = buf[:n]
			}

			if len(chunk) == 0 {
				break
			}

			if useCrypto {
				stream, e := aesutil.NewCTRFromNonce(s.CryptoKey[:], s.CryptoCtr[:], i)
				if e != nil {
					return 0, "", e
				}
				out := make([]byte, len(chunk))
				stream.XORKeyStream(out, chunk)
				chunk = out
			}

			if w != nil {
				if _, err = w.Write(chunk); err != nil {
					return 0, "", err
				}
			}
			if _, err = h.Write(chunk); err != nil {
				return 0, "", err
			}
			i += int64(len(chunk))
			decompressed += int64(len(chunk))
			readLogical += int64(len(chunk))
			if onProgress != nil {
				onProgress(readLogical, decompressed, ncaSize, "Decompress")
			}
		}
	}

	hexHash = fmt.Sprintf("%x", h.Sum(nil))
	if w != nil {
		if sw, ok := w.(io.Seeker); ok {
			var end int64
			end, err = sw.Seek(0, io.SeekCurrent)
			if err != nil {
				return 0, "", err
			}
			written = end - startOut
		}
	}
	return written, hexHash, nil
}
