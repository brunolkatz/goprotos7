package db_models

type IntVarDefinitions struct {
	Id           int64  `gorm:"primary_key;column:id;type:INT8;" json:"id" xml:"id" db:"id"`
	DbVariableId int64  `gorm:"column:db_variable_id;type:INT8;" json:"db_variable_id" xml:"db_variable_id" db:"db_variable_id"`
	Description  string `gorm:"column:description;type:TEXT;" json:"description" xml:"description" db:"description"`
	Value        int64  `gorm:"column:value;type:INT8;" json:"value" xml:"value" db:"value"`
}
