package goprotos7

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
)

func (c *Connection) AGWriteDB(dbNumber int, start int, size int, buffer []byte) (err error) {
	if size < 0 || start < 0 {
		return fmt.Errorf("invalid negative start/size")
	}
	if len(buffer) < size {
		return fmt.Errorf("buffer too small: got %d, need %d", len(buffer), size)
	}
	return c.setDBValue(uint16(dbNumber), TransportSizeByte, uint32(start), 0, buffer[:size])
}

// AGWriteDBWString writes an S7-1200 WSTRING to a simulated DB file.
// maxLength is the declared capacity in 16-bit characters (1..16382).
// The complete field is written, including zero-filled unused character slots.
// Only valid UTF-8 strings containing BMP characters are supported.
func (c *Connection) AGWriteDBWString(dbNumber, start, maxLength int, value string) error {
	const maxWStringLength = 16382
	if dbNumber < 0 || dbNumber > 65535 {
		return fmt.Errorf("invalid DB number: %d", dbNumber)
	}
	if start < 0 || uint64(start) > uint64(^uint32(0)) {
		return fmt.Errorf("invalid start: %d", start)
	}
	if maxLength < 1 || maxLength > maxWStringLength {
		return fmt.Errorf("invalid WSTRING capacity: %d", maxLength)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("WSTRING value is not valid UTF-8")
	}
	charCount := 0
	for _, r := range value {
		if r > 0xffff {
			return fmt.Errorf("WSTRING character U+%X is outside the Basic Multilingual Plane", r)
		}
		charCount++
	}
	if charCount > maxLength {
		return fmt.Errorf("WSTRING value has %d characters, exceeds capacity %d", charCount, maxLength)
	}
	size := 4 + 2*maxLength
	if uint64(start)+uint64(size) > uint64(^uint32(0)) {
		return fmt.Errorf("WSTRING field exceeds address range")
	}
	data := make([]byte, size)
	binary.BigEndian.PutUint16(data[0:2], uint16(maxLength))
	binary.BigEndian.PutUint16(data[2:4], uint16(charCount))
	pos := 4
	for _, r := range value {
		binary.BigEndian.PutUint16(data[pos:pos+2], uint16(r))
		pos += 2
	}
	return c.setDBValue(uint16(dbNumber), TransportSizeByte, uint32(start), 0, data)
}

func (c *Connection) setDBValue(dbNumber uint16, transportSize TransportSize, byteOffset uint32, bitOffset byte, data []byte) error {
	if c.options.BinFilesFolder == "" {
		panic("BinFilesFolder is not set")
	}

	dbFileName := fmt.Sprintf("DB%d.bin", dbNumber)
	dbPath := filepath.Join(c.options.BinFilesFolder, dbFileName)
	f, err := os.OpenFile(dbPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("error opening DB %s: %w", dbFileName, err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("error stating DB %s: %w", dbFileName, err)
	}

	switch transportSize {
	case TransportSizeBit:
		if len(data) < 1 {
			return fmt.Errorf("bit write requires at least one data byte")
		}
		if byteOffset >= uint32(stat.Size()) {
			return ErrorOutOfBounds
		}

		cur := make([]byte, 1)
		_, err = f.ReadAt(cur, int64(byteOffset))
		if err != nil {
			return fmt.Errorf("error reading current DB byte: %w", err)
		}

		mask := byte(1 << bitOffset)
		if data[0]&0x01 == 0x01 {
			cur[0] |= mask
		} else {
			cur[0] &^= mask
		}

		n, err := f.WriteAt(cur, int64(byteOffset))
		if err != nil {
			return fmt.Errorf("error writing DB bit value: %w", err)
		}
		if n != 1 {
			return fmt.Errorf("short write: wrote %d, expected 1", n)
		}
		return nil
	default:
		if byteOffset+uint32(len(data)) > uint32(stat.Size()) {
			return ErrorOutOfBounds
		}
		n, err := f.WriteAt(data, int64(byteOffset))
		if err != nil {
			return fmt.Errorf("error writing DB %s: %w", dbFileName, err)
		}
		if n != len(data) {
			return fmt.Errorf("short write: wrote %d, expected %d", n, len(data))
		}
		return nil
	}
}
