package goprotos7

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrorOutOfBounds = fmt.Errorf("out of bounds")
)

type TransportSize byte

const (
	TransportSizeBit         TransportSize = 0x03
	TransportSizeByte        TransportSize = 0x02
	TransportSizeChar        TransportSize = 0x09
	TransportSizeWord        TransportSize = 0x04
	TransportSizeInt         TransportSize = 0x05
	TransportSizeDWord       TransportSize = 0x06
	TransportSizeDInt        TransportSize = 0x07
	TransportSizeReal        TransportSize = 0x08
	TransportSizeS5Time      TransportSize = 0x0B
	TransportSizeTime        TransportSize = 0x0C
	TransportSizeDate        TransportSize = 0x0D
	TransportSizeTimeOfDay   TransportSize = 0x0F
	TransportSizeDateAndTime TransportSize = 0x10
)

var TransportSizeToByteLength = map[TransportSize]int{
	TransportSizeBit:         1, // 1 bit, but read at least 1 byte for file handling
	TransportSizeByte:        1,
	TransportSizeChar:        1,
	TransportSizeWord:        2,
	TransportSizeInt:         2,
	TransportSizeDWord:       4,
	TransportSizeDInt:        4,
	TransportSizeReal:        4,
	TransportSizeS5Time:      2,
	TransportSizeTime:        4,
	TransportSizeDate:        2,
	TransportSizeTimeOfDay:   4,
	TransportSizeDateAndTime: 8,
}

func (c *Connection) getDBValue(dbNumber uint16, transportSize byte, byteOffset uint32, bitOffset byte, length uint16) ([]byte, error) {
	db, err := c.getDB(dbNumber)
	if err != nil {
		return nil, err
	}

	_ = TransportSizeToByteLength[TransportSize(transportSize)]
	// get the offset value
	if byteOffset+uint32(length) > uint32(len(db)) {
		return nil, ErrorOutOfBounds
	}
	return db[byteOffset : byteOffset+uint32(length)], nil
}

func (c *Connection) getDB(dbNumber uint16) ([]byte, error) {
	if c.options.BinFilesFolder == "" {
		panic("BinFilesFolder is not set")
	}

	dbFileName := fmt.Sprintf("DB%d.bin", dbNumber)

	f, err := os.OpenFile(filepath.Join(c.options.BinFilesFolder, dbFileName), os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("error opening DB %s: %s", dbFileName, err)
	}
	binFile, err := os.ReadFile(f.Name())
	return binFile, err
}
