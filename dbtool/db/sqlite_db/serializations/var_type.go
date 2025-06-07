package gorm_serializers

import (
	"context"
	"errors"
	"github.com/brunolkatz/goprotos7/dbtool"
	"gorm.io/gorm/schema"
	"reflect"
)

var (
	VarTypeEnumTypeTagName         = "var_type_serializer" // Used when you register the serializer
	ErrVarTypeEnumTypeNotSupported = errors.New("var_type enum type not supported")
)

type VarTypeEnumSerializer struct{}

func (VarTypeEnumSerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue interface{}) (interface{}, error) {
	if fieldValue == nil {
		return nil, nil
	}

	if t, ok := fieldValue.(dbtool.VarType); ok {
		return dbtool.VarTypeDepara[t], nil
	}

	return nil, ErrVarTypeEnumTypeNotSupported
}

func (VarTypeEnumSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) (err error) {
	var t dbtool.VarType

	if dbValue != nil {
		switch v := dbValue.(type) {
		case string:
			t = dbtool.VarTypePara[v]
		default:
			return ErrVarTypeEnumTypeNotSupported
		}

		field.ReflectValueOf(ctx, dst).Set(reflect.ValueOf(t))
	}

	return
}
