package db_models

import "time"

type HeartbeatType string

const (
	HeartbeatTypeWhenTrueSetFalse HeartbeatType = "WHEN_TRUE_SET_FALSE"
	HeartbeatTypeWhenFalseSetTrue HeartbeatType = "WHEN_FALSE_SET_TRUE"
)

type HeartbeatRegistration struct {
	ID             int64         `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DBNumber       int64         `gorm:"column:db_number;not null" json:"db_number"`
	VariableName   string        `gorm:"column:variable_name;not null" json:"variable_name"`
	Address        string        `gorm:"column:address;not null;uniqueIndex" json:"address"`
	StartOnConnect bool          `gorm:"column:start_on_connect;not null;default:true" json:"start_on_connect"`
	Enabled        bool          `gorm:"column:enabled;not null;default:true" json:"enabled"`
	HeartbeatType  HeartbeatType `gorm:"column:heartbeat_type;not null" json:"heartbeat_type"`
	AlarmSeconds   int64         `gorm:"column:alarm_seconds;not null;default:5" json:"alarm_seconds"`
	LastValue      *bool         `gorm:"column:last_value" json:"last_value,omitempty"`
	LastCheckAt    *time.Time    `gorm:"column:last_check_at" json:"last_check_at,omitempty"`
	LastChangeAt   *time.Time    `gorm:"column:last_change_at" json:"last_change_at,omitempty"`
	IsFailing      bool          `gorm:"column:is_failing;not null;default:false" json:"is_failing"`
	LastError      string        `gorm:"column:last_error" json:"last_error,omitempty"`
	CreatedAt      time.Time     `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time     `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (HeartbeatRegistration) TableName() string {
	return "heartbeat_registrations"
}
