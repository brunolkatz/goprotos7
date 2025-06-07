package sql_lite_db

import (
	"context"
	"fmt"
	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
)

func (d *DB) GetDBVarById(ctx context.Context, id int64) (*db_models.DbVariable, error) {
	var ret db_models.DbVariable
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariable{}).
		Preload("StaticVarDefinitions").
		Where("id = ?", id).
		First(&ret).Error
	if err != nil {
		return nil, err
	}
	if ret.Id == 0 {
		return nil, fmt.Errorf("no variable found")
	}
	ret.UpdateIsSelected()
	return &ret, nil
}

func (d *DB) GetLatestVariable(dbNumber int64) (*db_models.DbVariable, error) {
	var ret db_models.DbVariable
	err := d.DbConn.
		Model(db_models.DbVariable{}).
		Preload("StaticVarDefinitions").
		Where("db_number = ?", dbNumber).
		Order("byte_offset DESC").
		Limit(1).
		First(&ret).Error
	if err != nil {
		return nil, err
	}
	if ret.Id == 0 {
		return nil, nil // No variables found
	}
	return &ret, nil
}

func (d *DB) GetDBNumbers(ctx context.Context) ([]uint32, error) {
	type Ret struct {
		DbNumber uint32 `gorm:"column:db_number" db:"db_number" json:"db_number"`
	}
	var ret []*Ret
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariable{}).
		Select("db_number").
		Group("db_number").
		Order("db_number ASC").
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	if len(ret) == 0 {
		return make([]uint32, 0), nil
	}
	var dbNumbers []uint32
	for _, r := range ret {
		dbNumbers = append(dbNumbers, r.DbNumber)
	}
	return dbNumbers, nil
}

func (d *DB) GetVariables(ctx context.Context, dbNumber int32) ([]*db_models.DbVariable, error) {
	var ret []*db_models.DbVariable
	query := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariable{}).
		Preload("StaticVarDefinitions")
	if dbNumber >= 0 {
		query = query.Where("db_number = ?", dbNumber)
	}
	query = query.Order("byte_offset DESC")
	err := query.Find(&ret).Error
	if err != nil {
		return nil, err
	}

	for _, v := range ret {
		v.UpdateIsSelected()
	}

	return ret, nil
}

func (d *DB) GetDbVariables_ASC(ctx context.Context, dbNumber uint32) ([]*db_models.DbVariable, error) {
	var ret []*db_models.DbVariable
	err := d.DbConn.
		WithContext(ctx).
		Model(db_models.DbVariable{}).
		Preload("StaticVarDefinitions").
		Where("db_number = ?", dbNumber).
		Order("byte_offset ASC").
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	return ret, nil
}
