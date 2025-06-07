package gorm_serializers

import (
	"context"
	"errors"
	"github.com/brunolkatz/goprotos7"
	"gorm.io/gorm/schema"
	"reflect"
)

var (
	DataTypeEnumTypeTagName         = "data_type_serializer" // Used when you register the serializer
	ErrDataTypeEnumTypeNotSupported = errors.New("data_type enum type not supported")
)

type DataTypeEnumSerializer struct{}

func (DataTypeEnumSerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue interface{}) (interface{}, error) {
	if fieldValue == nil {
		return nil, nil
	}

	if t, ok := fieldValue.(goprotos7.DataType); ok {
		return goprotos7.DataTypeStr[t], nil
	}

	return nil, ErrDataTypeEnumTypeNotSupported
}

func (DataTypeEnumSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) (err error) {
	var t goprotos7.DataType

	if dbValue != nil {
		switch v := dbValue.(type) {
		case string:
			t = goprotos7.DataTypeDepara[v]
		default:
			return ErrDataTypeEnumTypeNotSupported
		}

		field.ReflectValueOf(ctx, dst).Set(reflect.ValueOf(t))
	}

	return
}
