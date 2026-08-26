package db_models

type PLCWatchList struct {
	Source    string `gorm:"column:source;type:TEXT;primaryKey" json:"source"`
	DBNumber  int32  `gorm:"column:db_number;primaryKey" json:"db_number"`
	Addresses string `gorm:"column:addresses;type:TEXT;not null;default:''" json:"addresses"`
}

func (PLCWatchList) TableName() string {
	return "plc_watch_lists"
}
