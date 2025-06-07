package build_datab

import (
	"encoding/binary"
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
	"math"
	"os"
	"path/filepath"
)

// BuildDataBlocks - Build and create the database binary file to store the data blocks
func BuildDataBlocks(path string, variables []*db_models.DbVariable) error {
	if variables == nil || len(variables) == 0 {
		return fmt.Errorf("no variables provided")
	}

	buff, err := createBuffer(variables)
	if err != nil {
		return fmt.Errorf("error creating buffer: %v", err)
	}

	err = saveBinaryFile(path, buff)
	if err != nil {
		return fmt.Errorf("error saving binary file: %v", err)
	}
	return nil
}

func WriteVariableToFile(path string, dbVariable *db_models.DbVariable) ([]byte, error) {
	if dbVariable == nil {
		return nil, fmt.Errorf("dbVariable is nil")
	}

	if path == "" {
		return nil, fmt.Errorf("path is empty")
	}

	buff, err := writeToFile(path, dbVariable)
	if err != nil {
		return nil, fmt.Errorf("error writing to file: %v", err)
	}

	return buff, nil
}

// createBuffer - This function will initialize all variables with the default value
func createBuffer(variables []*db_models.DbVariable) ([]byte, error) {

	var maxByte int
	for _, v := range variables {
		dt := v.DataType
		if bSize, ok := goprotos7.DataTypeSize[dt]; !ok {
			return nil, fmt.Errorf("unknown data size (id: %d) type %s", v.Id, v.DataType)
		} else {

			// We need to verify if the string has the length defined
			if dt == goprotos7.STRING && v.Length == nil {
				return nil, fmt.Errorf("unknown data length (id: %d) type %s", v.Id, v.DataType)
			}

			// String is a special case because they store the length of the string and
			// the actual string size
			if dt == goprotos7.STRING && v.Length != nil {
				maxByte = max(maxByte, int(v.ByteOffset)+int(*v.Length)+int(bSize))
			} else {
				maxByte = max(maxByte, int(v.ByteOffset)+int(bSize))
			}
		}
	}

	// Create a buf to hold the data blocks
	buf := make([]byte, maxByte)

	for _, v := range variables {
		switch v.DataType {
		case goprotos7.BOOL:
			bValue := buf[v.ByteOffset]
			if v.BitOffset != nil {
				if v.BoolVal != nil {
					b := uint8(0)
					if *v.BoolVal {
						b = 1
					}
					bValue |= b << *v.BitOffset
				} else {
					bValue |= uint8(0) << *v.BitOffset
				}
			} else {
				bValue = uint8(0) // Set a default value
			}
			buf[v.ByteOffset] = bValue // Store the updated byte value
		case goprotos7.BYTE, goprotos7.USINT:
			if v.IntVal != nil {
				if *v.IntVal > math.MaxUint8 {
					return nil, fmt.Errorf("%s value is greater than %d (id: %d) type %s", v.DataType, math.MaxUint8, v.Id, v.DataType)
				}
				buf[v.ByteOffset] = byte(*v.IntVal)
			} else {
				buf[v.ByteOffset] = 0 // default 0
			}
		case goprotos7.WORD, goprotos7.UINT: // uint16 variable with 2 bytes - DBW
			if v.IntVal != nil {
				if *v.IntVal > math.MaxUint16 {
					return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", v.DataType, math.MaxUint16, v.Id)
				}
				binary.BigEndian.PutUint16(buf[v.ByteOffset:], uint16(*v.IntVal))
			} else {
				binary.BigEndian.PutUint16(buf[v.ByteOffset:], uint16(0))
			}
		case goprotos7.DWORD, goprotos7.UDINT: // uint32 variable with 4 bytes
			if v.IntVal != nil {
				if *v.IntVal > math.MaxUint32 {
					return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", v.DataType, math.MaxUint32, v.Id)
				}
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], uint32(*v.IntVal))
			} else {
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], uint32(0))
			}
		case goprotos7.LWORD, goprotos7.ULINT: // uint64 variable with 8 bytes
			if v.IntVal != nil {
				if uint64(*v.IntVal) > math.MaxUint64 {
					return nil, fmt.Errorf(" type %s value is greater than math.MaxUint64 (id: %d)", v.DataType, v.Id)
				}
				binary.BigEndian.PutUint64(buf[v.ByteOffset:], uint64(*v.IntVal))
			} else {
				binary.BigEndian.PutUint64(buf[v.ByteOffset:], uint64(0))
			}
		case goprotos7.SINT:
			if v.IntVal != nil {
				if *v.IntVal > 127 {
					return nil, fmt.Errorf("SINT value is greater than 127 (id: %d) type %s", v.Id, v.DataType)
				}
				buf[v.ByteOffset] = byte(*v.IntVal)
			} else {
				buf[v.ByteOffset] = uint8(0) // default 0
			}
		case goprotos7.INT: // Signed 16-bit integer
			if v.IntVal != nil {
				if *v.IntVal > math.MaxInt16 {
					return nil, fmt.Errorf("INT value is greater than %d (id: %d) type %s", math.MaxInt16, v.Id, v.DataType)
				}
				binary.BigEndian.PutUint16(buf[v.ByteOffset:], uint16(*v.IntVal))
			} else {
				binary.BigEndian.PutUint16(buf[v.ByteOffset:], uint16(0))
			}
		case goprotos7.DINT: // Signed 32-bit integer
			if v.IntVal != nil {
				if *v.IntVal > math.MaxInt32 {
					return nil, fmt.Errorf("DINT value is greater than %d (id: %d) type %s", math.MaxInt32, v.Id, v.DataType)
				}
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], uint32(*v.IntVal))
			} else {
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], uint32(0))
			}
		case goprotos7.LINT: // int64 variable with 8 bytes, so we need the sign bit as well
			if v.IntVal != nil {
				if *v.IntVal > math.MaxInt64 {
					return nil, fmt.Errorf("LINT value is greater than %d (id: %d) type %s", math.MaxInt64, v.Id, v.DataType)
				}
				binary.BigEndian.PutUint64(buf[v.ByteOffset:], uint64(*v.IntVal))
			} else {
				// fill with 0
				copy(buf[v.ByteOffset:v.ByteOffset+8], make([]byte, 8))
			}
		case goprotos7.REAL: // 32-bit IEEE 754 floating point
			if v.FloatVal != nil {
				if *v.FloatVal > math.MaxFloat32 {
					return nil, fmt.Errorf("REAL value is greater than %f (id: %d) type %s", math.MaxFloat32, v.Id, v.DataType)
				}
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], math.Float32bits(float32(*v.FloatVal)))
			} else {
				binary.BigEndian.PutUint32(buf[v.ByteOffset:], 0) // default 0
			}
		case goprotos7.LREAL: // 64-bit IEEE 754 floating point
			if v.FloatVal != nil {
				if *v.FloatVal > math.MaxFloat64 {
					return nil, fmt.Errorf("LREAL value is greater than %f (id: %d) type %s", math.MaxFloat64, v.Id, v.DataType)
				}
				binary.BigEndian.PutUint64(buf[v.ByteOffset:], math.Float64bits(*v.FloatVal))
			} else {
				binary.BigEndian.PutUint64(buf[v.ByteOffset:], 0) // default 0
			}
		case goprotos7.CHAR:
			if v.StringVal != nil {
				if len(*v.StringVal) > 1 {
					return nil, fmt.Errorf("CHAR value is greater than 1 (id: %d) type %s", v.Id, v.DataType)
				}
				buf[v.ByteOffset] = (*v.StringVal)[0]
			} else {
				buf[v.ByteOffset] = 0
			}
		case goprotos7.STRING:
			if v.Length != nil {
				l := *v.Length
				// Store the length of the string
				buf[v.ByteOffset] = l
				// Store all remain bytes
				if v.StringVal == nil {
					// no value, set the actual length to 0
					buf[v.ByteOffset+1] = 0
					copy(buf[v.ByteOffset+2:l-1], make([]byte, l))
				} else {
					if len(*v.StringVal) > int(l) {
						return nil, fmt.Errorf("string length is greater than the defined length (id: %d) type %s", v.Id, v.DataType)
					}
					buf[v.ByteOffset+1] = byte(len(*v.StringVal)) // Set the actual length
					to := v.ByteOffset + 2 + int64(l)
					copy(buf[v.ByteOffset+2:to], *v.StringVal)
				}
			}
		}
	}

	// Add logic to fill the buf with data blocks based on the variables,
	// For example, you can iterate over the variables and append their data to the buf

	return buf, nil
}

func writeToFile(path string, dbVariable *db_models.DbVariable) ([]byte, error) {

	buff, err := getBinFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading binary file: %v", err)
	}

	switch dbVariable.DataType {
	case goprotos7.BOOL: // Boolean variable
		bVal := buff[dbVariable.ByteOffset]
		if dbVariable.BitOffset != nil {
			if dbVariable.BoolVal != nil {
				b := uint8(0)
				if *dbVariable.BoolVal {
					b = 1
				}
				if b == 1 {
					bVal |= b << *dbVariable.BitOffset
				} else {
					bVal &^= 1 << *dbVariable.BitOffset // Clear the bit at the specified offset
				}
			} else {
				bVal &^= 1 << *dbVariable.BitOffset
			}
		} else {
			bVal = uint8(0) // Set a default value
		}
		buff[dbVariable.ByteOffset] = bVal // Store the updated byte value
	case goprotos7.BYTE: // 8-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint8 {
				return nil, fmt.Errorf("%s value is greater than %d (id: %d) type %s", dbVariable.DataType, math.MaxUint8, dbVariable.Id, dbVariable.DataType)
			}
			buff[dbVariable.ByteOffset] = byte(*dbVariable.IntVal)
		} else {
			buff[dbVariable.ByteOffset] = 0 // default 0
		}
	case goprotos7.WORD: // 16-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint16 {
				return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", dbVariable.DataType, math.MaxUint16, dbVariable.Id)
			}
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(0))
		}
	case goprotos7.DWORD: // 32-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint32 {
				return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", dbVariable.DataType, math.MaxUint32, dbVariable.Id)
			}
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(0))
		}
	case goprotos7.LWORD: // 64-bit unsigned integer
		if dbVariable.IntVal != nil {
			if uint64(*dbVariable.IntVal) > math.MaxUint64 {
				return nil, fmt.Errorf(" type %s value is greater than math.MaxUint64 (id: %d)", dbVariable.DataType, dbVariable.Id)
			}
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(0))
		}
	case goprotos7.SINT: // 8-bit signed integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > 127 {
				return nil, fmt.Errorf("SINT value is greater than 127 (id: %d) type %s", dbVariable.Id, dbVariable.DataType)
			}
			buff[dbVariable.ByteOffset] = byte(*dbVariable.IntVal)
		} else {
			buff[dbVariable.ByteOffset] = uint8(0) // default 0
		}
	case goprotos7.USINT: // 8-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint8 {
				return nil, fmt.Errorf("%s value is greater than %d (id: %d) type %s", dbVariable.DataType, math.MaxUint8, dbVariable.Id, dbVariable.DataType)
			}
			buff[dbVariable.ByteOffset] = byte(*dbVariable.IntVal)
		} else {
			buff[dbVariable.ByteOffset] = 0 // default 0
		}
	case goprotos7.INT: // 16-bit signed integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxInt16 {
				return nil, fmt.Errorf("INT value is greater than %d (id: %d) type %s", math.MaxInt16, dbVariable.Id, dbVariable.DataType)
			}
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(0))
		}
	case goprotos7.UINT: // 16-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint16 {
				return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", dbVariable.DataType, math.MaxUint16, dbVariable.Id)
			}
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint16(buff[dbVariable.ByteOffset:], uint16(0))
		}
	case goprotos7.DINT: // 32-bit signed integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxInt32 {
				return nil, fmt.Errorf("DINT value is greater than %d (id: %d) type %s", math.MaxInt32, dbVariable.Id, dbVariable.DataType)
			}
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(0))
		}
	case goprotos7.UDINT: // 32-bit unsigned integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxUint32 {
				return nil, fmt.Errorf("type %s value is greater than %d (id: %d)", dbVariable.DataType, math.MaxUint32, dbVariable.Id)
			}
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], uint32(0))
		}
	case goprotos7.LINT: // 64-bit signed integer
		if dbVariable.IntVal != nil {
			if *dbVariable.IntVal > math.MaxInt64 {
				return nil, fmt.Errorf("LINT value is greater than %d (id: %d) type %s", math.MaxInt64, dbVariable.Id, dbVariable.DataType)
			}
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(0)) // default 0
		}
	case goprotos7.ULINT: // 64-bit unsigned integer
		if dbVariable.IntVal != nil {
			if uint64(*dbVariable.IntVal) > math.MaxUint64 {
				return nil, fmt.Errorf(" type %s value is greater than math.MaxUint64 (id: %d)", dbVariable.DataType, dbVariable.Id)
			}
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(*dbVariable.IntVal))
		} else {
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], uint64(0)) // default 0
		}
	case goprotos7.REAL: // 32-bit IEEE 754 floating point
		if dbVariable.FloatVal != nil {
			if *dbVariable.FloatVal > math.MaxFloat32 {
				return nil, fmt.Errorf("REAL value is greater than %f (id: %d) type %s", math.MaxFloat32, dbVariable.Id, dbVariable.DataType)
			}
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], math.Float32bits(float32(*dbVariable.FloatVal)))
		} else {
			binary.BigEndian.PutUint32(buff[dbVariable.ByteOffset:], 0) // default 0
		}
	case goprotos7.LREAL: // 64-bit IEEE 754 floating point
		if dbVariable.FloatVal != nil {
			if *dbVariable.FloatVal > math.MaxFloat64 {
				return nil, fmt.Errorf("LREAL value is greater than math.MaxFloat64 (id: %d) type %s", dbVariable.Id, dbVariable.DataType)
			}
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], math.Float64bits(*dbVariable.FloatVal))
		} else {
			binary.BigEndian.PutUint64(buff[dbVariable.ByteOffset:], 0) // default 0
		}
	case goprotos7.CHAR: // 8-bit character
		if dbVariable.StringVal != nil {
			if len(*dbVariable.StringVal) > 1 {
				return nil, fmt.Errorf("CHAR value is greater than 1 (id: %d) type %s", dbVariable.Id, dbVariable.DataType)
			}
			buff[dbVariable.ByteOffset] = (*dbVariable.StringVal)[0]
		} else {
			buff[dbVariable.ByteOffset] = 0
		}
	case goprotos7.STRING: // String variable
		if dbVariable.Length != nil {
			l := *dbVariable.Length
			if l > buff[dbVariable.ByteOffset] {
				return nil, fmt.Errorf("string length is greater than the already defined length (id: %d) type %s", dbVariable.Id, dbVariable.DataType)
			}
			// Store all remain bytes
			if dbVariable.StringVal == nil {
				// no value, set the actual length to 0
				buff[dbVariable.ByteOffset+1] = 0
				copy(buff[dbVariable.ByteOffset+2:l-1], make([]byte, l))
			} else {
				if len(*dbVariable.StringVal) > int(l) {
					return nil, fmt.Errorf("string length is greater than the defined length (id: %d) type %s", dbVariable.Id, dbVariable.DataType)
				}
				buff[dbVariable.ByteOffset+1] = byte(len(*dbVariable.StringVal)) // Set the actual length
				to := dbVariable.ByteOffset + 2 + int64(l)
				copy(buff[dbVariable.ByteOffset+2:to], *dbVariable.StringVal)
			}
		} else {
			return nil, fmt.Errorf("STRING variable (id: %d) does not have a defined length", dbVariable.Id)
		}
	}

	// Save the updated buffer back to the file
	err = saveBinaryFile(path, buff)
	if err != nil {
		return nil, fmt.Errorf("error saving binary file: %v", err)
	}

	return buff, nil
}

func getBinFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Printf("error closing file %s: %s\n", path, err)
		}
	}(f)
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	b := make([]byte, fi.Size())
	_, err = f.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SaveBinaryFile writes the given buffer to a file at the specified path.
func saveBinaryFile(path string, buf []byte) error {

	// Extract the path and verify if the directory exists
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// open the file and truncate it if it exists
	targetFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// Write all the buffer to the new file
	if _, err = targetFile.Write(buf); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}
