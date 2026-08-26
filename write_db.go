package goprotos7

import (
	"fmt"
	"os"
	"path/filepath"
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
