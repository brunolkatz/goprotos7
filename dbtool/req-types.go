package dbtool

import "github.com/brunolkatz/goprotos7"

type ListFields struct {
	Description string
	IntValue    *int64
	FloatValue  *int64
	BoolValue   *bool
	BitOffset   *int64 // Used when data-type is BOOL, the bit offset of the BOOL value.
	StaticType  StaticType
}

type CreateVarRequest struct {
	DBNumber    int64              `json:"db_number"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	DataType    goprotos7.DataType `json:"data_type"`

	IntVal   *int64   `json:"int_val"`
	FloatVal *float64 `json:"real_val"`

	StringVal *string `json:"str_val"`
	StrLength *uint8  `json:"str_length"` // Used when data-type is STRING, the length of the string.

	BoolVal *bool `json:"bool_val"`

	ListFields []*ListFields `json:"list_fields,omitempty"` // Store the int values when data-type is INT, DINT, LINT, etc.
}
