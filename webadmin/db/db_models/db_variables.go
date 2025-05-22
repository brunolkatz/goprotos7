package db_models

type DbVariables struct {
	Id          int64  `gorm:"primary_key;column:id;type:INT8;" json:"id" xml:"id" db:"id"`
	DbNumber    int64  `gorm:"column:db_number;type:INT8;" json:"db_number" xml:"db_number" db:"db_number"`
	Name        string `gorm:"column:name;type:TEXT;" json:"name" xml:"name" db:"name"`
	DataType    string `gorm:"column:data_type;type:TEXT;" json:"data_type" xml:"data_type" db:"data_type"`
	ByteOffset  int64  `gorm:"column:byte_offset;type:INT8;" json:"byte_offset" xml:"byte_offset" db:"byte_offset"`
	BitOffset   *int64 `gorm:"column:bit_offset;type:INT8;" json:"bit_offset" xml:"bit_offset" db:"bit_offset"`
	Length      *uint8 `gorm:"column:length;type:INT8;" json:"length" xml:"length" db:"length"`
	Description string `gorm:"column:description;type:TEXT;" json:"description" xml:"description" db:"description"`

	IntVal    *int64   `gorm:"column:int_val;type:INT8;" json:"int_val" xml:"int_val" db:"int_val"`
	FloatVal  *float64 `gorm:"column:real_val;type:FLOAT;" json:"real_val" xml:"real_val" db:"real_val"`
	StringVal *string  `gorm:"column:str_val;type:TEXT;" json:"str_val" xml:"str_val" db:"str_val"`
	BoolVal   *bool    `gorm:"column:bool_val;type:BOOL;" json:"bool_val" xml:"bool_val" db:"bool_val"`
}
