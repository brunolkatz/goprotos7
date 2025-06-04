package dbtool

import "github.com/brunolkatz/goprotos7"

type IntFields struct {
	Description string
	Value       int64
}

type CreateVarRequest struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	DataType    goprotos7.DataType `json:"data_type"`

	IntVal    *int64   `json:"int_val"`
	FloatVal  *float64 `json:"real_val"`
	StringVal *string  `json:"str_val"`
	BoolVal   *bool    `json:"bool_val"`

	IntFields []*IntFields `json:"int_fields,omitempty"` // Store the int values when data-type is INT, DINT, LINT, etc.
}
