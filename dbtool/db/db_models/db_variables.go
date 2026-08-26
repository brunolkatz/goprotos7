package db_models

import (
	"fmt"
	"github.com/brunolkatz/goprotos7"
	"github.com/brunolkatz/goprotos7/dbtool"
)

type DbVariable struct {
	Id          int64              `gorm:"primary_key;column:id;type:INT8;" json:"id" xml:"id" db:"id"`
	DbNumber    int64              `gorm:"column:db_number;type:INT8;" json:"db_number" xml:"db_number" db:"db_number"`
	Name        string             `gorm:"column:name;type:TEXT;" json:"name" xml:"name" db:"name"`
	DataType    goprotos7.DataType `gorm:"column:data_type;type:TEXT;serializer:data_type_serializer" json:"data_type" xml:"data_type" db:"data_type"`
	ByteOffset  int64              `gorm:"column:byte_offset;type:INT8;" json:"byte_offset" xml:"byte_offset" db:"byte_offset"`
	BitOffset   *int64             `gorm:"column:bit_offset;type:INT8;" json:"bit_offset" xml:"bit_offset" db:"bit_offset"`
	Length      *uint8             `gorm:"column:length;type:INT8;" json:"length" xml:"length" db:"length"`
	Description string             `gorm:"column:description;type:TEXT;" json:"description" xml:"description" db:"description"`

	IntVal    *int64   `gorm:"column:int_val;type:INT8;" json:"int_val" xml:"int_val" db:"int_val"`
	FloatVal  *float64 `gorm:"column:real_val;type:FLOAT;" json:"real_val" xml:"real_val" db:"real_val"`
	StringVal *string  `gorm:"column:str_val;type:TEXT;" json:"str_val" xml:"str_val" db:"str_val"`
	BoolVal   *bool    `gorm:"column:bool_val;type:BOOL;" json:"bool_val" xml:"bool_val" db:"bool_val"`

	VarType dbtool.VarType `gorm:"column:var_type;type:TEXT;serializer:var_type_serializer" json:"var_type" xml:"var_type" db:"var_type"` // e.g., "STATIC", "LIST", etc. see VarType at models.go file

	StaticVarDefinitions []*StaticVarDefinition `gorm:"foreignKey:DbVariableId;references:Id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"static_var_definitions,omitempty" `
}

func (d *DbVariable) UpdateIsSelected() {
	if d == nil {
		return
	}
	if d.StaticVarDefinitions != nil {
		// Reset all static variable definitions
		for _, s := range d.StaticVarDefinitions {
			s.IsSelected = false
		}
	OUTFOR:
		for _, s := range d.StaticVarDefinitions {
			switch s.StaticType {
			case "BOOL":
				if s.IntValue != nil && d.BoolVal != nil {
					if *s.IntValue == 1 && *d.BoolVal == true {
						s.IsSelected = true
						break OUTFOR
					}
					if *s.IntValue == 0 && *d.BoolVal == false {
						s.IsSelected = true
						break OUTFOR
					}
				}
			case "INT":
				if s.IntValue != nil && d.IntVal != nil {
					if *s.IntValue == *d.IntVal {
						s.IsSelected = true
						break OUTFOR
					}
				}
			}
		}
	}
}

func (d *DbVariable) ToDBAddress() string {
	addr := fmt.Sprintf("DB%d", d.DbNumber)
	switch d.DataType {
	case goprotos7.BOOL: // 1 bit boolean -> bool
		if d.BitOffset != nil {
			addr += fmt.Sprintf(".DBX%d.%d", d.ByteOffset, *d.BitOffset)
		} else {
			addr += fmt.Sprintf(".DBX%d.0", d.ByteOffset)
		}
	case goprotos7.BYTE: // 1 byte unsigned integer -> uint8
		addr += fmt.Sprintf(".DBB%d", d.ByteOffset)
	case goprotos7.WORD: // 2 bytes unsigned integer -> uint16
		addr += fmt.Sprintf(".DBW%d", d.ByteOffset)
	case goprotos7.DWORD: // 4 bytes unsigned integer -> uint32
		addr += fmt.Sprintf(".DBDW%d", d.ByteOffset)
	case goprotos7.LWORD: // 8 bytes unsigned integer -> uint64
		addr += fmt.Sprintf(".DBL%d", d.ByteOffset)
	case goprotos7.SINT: // 1 byte signed integer -> int8
		addr += fmt.Sprintf(".DBB%d", d.ByteOffset)
	case goprotos7.USINT: // 1 byte unsigned integer -> uint8
		addr += fmt.Sprintf(".DBB%d", d.ByteOffset)
	case goprotos7.INT: // 2 bytes signed integer -> int16
		addr += fmt.Sprintf(".DBW%d", d.ByteOffset)
	case goprotos7.UINT: // 2 bytes unsigned integer -> uint16
		addr += fmt.Sprintf(".DBW%d", d.ByteOffset)
	case goprotos7.DINT: // 4 bytes signed integer -> int32
		addr += fmt.Sprintf(".DBD%d", d.ByteOffset)
	case goprotos7.UDINT: // 4 bytes unsigned integer -> uint32
		addr += fmt.Sprintf(".DBD%d", d.ByteOffset)
	case goprotos7.LINT: // 8 bytes signed integer -> int64
		addr += fmt.Sprintf(".DBL%d", d.ByteOffset)
	case goprotos7.ULINT: // 8 bytes unsigned integer -> uint64
		addr += fmt.Sprintf(".DBL%d", d.ByteOffset)
	case goprotos7.REAL: // 4 bytes floating point number -> float32
		addr += fmt.Sprintf(".DBD%d", *d.BitOffset)
	case goprotos7.LREAL: // 8 bytes floating point number -> float64
		addr += fmt.Sprintf(".DBL%d", d.ByteOffset)
	case goprotos7.CHAR: // 1 byte character -> char
		addr += fmt.Sprintf(".DBB%d", d.ByteOffset)
	case goprotos7.STRING:
		if d.Length == nil {
			addr += fmt.Sprintf(".DBS%d.0", d.ByteOffset) // No length specified, default to 0
		} else {
			addr += fmt.Sprintf(".DBS%d.%d", d.ByteOffset, *d.Length)
		}
	}
	return addr
}

func (d *DbVariable) FmtValue() string {
	if d == nil {
		return "data type not supported"
	}
	switch d.DataType {
	case goprotos7.DT_UNSED:
		return "data type unused"
	case goprotos7.STRING, goprotos7.CHAR:
		if d.StringVal != nil {
			return *d.StringVal
		}
		return "string value not set"
	case goprotos7.BOOL:
		if d.BoolVal != nil {
			if *d.BoolVal {
				return "true"
			}
			return "false"
		}
		return "boolean value not set"
	case goprotos7.BYTE, goprotos7.WORD, goprotos7.DWORD, goprotos7.LWORD, goprotos7.SINT, goprotos7.USINT, goprotos7.INT, goprotos7.UINT, goprotos7.DINT, goprotos7.UDINT, goprotos7.LINT:
		if d.IntVal != nil {
			return fmt.Sprintf("%d", *d.IntVal)
		}
		return "integer value not set"
	case goprotos7.REAL, goprotos7.LREAL:
		if d.FloatVal != nil {
			return fmt.Sprintf("%f", *d.FloatVal)
		}
	default:
		return fmt.Sprintf("data type %s not supported", d.DataType.String())
	}
	return "value not set"
}

func (d *DbVariable) TableName() string {
	return "db_variables"
}
