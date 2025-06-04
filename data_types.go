package goprotos7

type DataType uint32

func (d DataType) String() string {
	if _, ok := DataTypeStr[d]; ok {
		return DataTypeStr[d]
	}
	return "Unknown"
}

const (
	DT_UNSED DataType = 999 // Unused data type, for future use or error handling
)

// TODO: add support for other datatypes
const (
	BOOL DataType = iota
	BYTE
	WORD
	DWORD
	LWORD
	SINT
	USINT
	INT
	UINT
	DINT
	UDINT
	LINT
	ULINT
	REAL
	LREAL
	CHAR
	STRING
)

var (
	OrderedDataTypes = []DataType{
		BOOL,
		BYTE,
		WORD,
		DWORD,
		LWORD,
		SINT,
		USINT,
		INT,
		UINT,
		DINT,
		UDINT,
		LINT,
		ULINT,
		REAL,
		LREAL,
		CHAR,
		STRING,
	}
	DataTypeStr = map[DataType]string{
		BOOL:   "BOOL",
		BYTE:   "BYTE",
		WORD:   "WORD",
		DWORD:  "DWORD",
		LWORD:  "LWORD",
		SINT:   "SINT",
		USINT:  "USINT",
		INT:    "INT",
		UINT:   "UINT",
		DINT:   "DINT",
		UDINT:  "UDINT",
		LINT:   "LINT",
		ULINT:  "ULINT",
		REAL:   "REAL",
		LREAL:  "LREAL",
		CHAR:   "CHAR",
		STRING: "STRING[n]",
	}
	DataTypeDepara = map[string]DataType{
		"BOOL":      BOOL,
		"BYTE":      BYTE,
		"WORD":      WORD,
		"DWORD":     DWORD,
		"LWORD":     LWORD,
		"SINT":      SINT,
		"USINT":     USINT,
		"INT":       INT,
		"UINT":      UINT,
		"DINT":      DINT,
		"UDINT":     UDINT,
		"LINT":      LINT,
		"ULINT":     ULINT,
		"REAL":      REAL,
		"LREAL":     LREAL,
		"CHAR":      CHAR,
		"STRING[n]": STRING,
		"STRING":    STRING,
	}
	DataTypeSize = map[DataType]uint8{
		BOOL:   1,
		BYTE:   1,
		WORD:   2,
		DWORD:  4,
		LWORD:  8,
		SINT:   1,
		USINT:  1,
		INT:    2,
		UINT:   2,
		DINT:   4,
		UDINT:  4,
		LINT:   8,
		ULINT:  8,
		REAL:   4,
		LREAL:  8,
		CHAR:   1,
		STRING: 2, // The STRING data type stores 2 + n bytes, where n is the length of the string.
	}
)
