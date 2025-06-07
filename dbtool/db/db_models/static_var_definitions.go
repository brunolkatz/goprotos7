package db_models

import "github.com/brunolkatz/goprotos7/dbtool"

// StaticVarDefinition - Stores the int and float static values definitions, is used mainly to render the Buttons to "change" static values;
// e.g.: of buttons to toggle the "equipment status"
type StaticVarDefinition struct {
	Id           int64             `gorm:"primary_key;column:id;type:INT8;" json:"id" xml:"id" db:"id"`
	DbVariableId int64             `gorm:"column:db_variable_id;type:INT8;" json:"db_variable_id" xml:"db_variable_id" db:"db_variable_id"`
	Description  string            `gorm:"column:description;type:TEXT;" json:"description" xml:"description" db:"description"`
	IntValue     *int64            `gorm:"column:int_value;type:INT8;" json:"int_value" xml:"int_value" db:"int_value"`
	FloatValue   *float64          `gorm:"column:float_value;type:REAL;" json:"float_value" xml:"float_value" db:"float_value"`
	StaticType   dbtool.StaticType `gorm:"column:static_type;type:TEXT;" json:"static_type" xml:"static_type" db:"static_type"` // TODO: add the serializer for this field

	IsSelected bool `gorm:"-" db:"-" json:"is_selected"` // Indicates if this static var definition is selected in the UI, not stored in the database
}

func (d *StaticVarDefinition) TableName() string {
	return "static_var_definitions"
}
