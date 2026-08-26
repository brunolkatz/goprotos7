package sql_lite_db

import (
	"context"
	"strings"
	"time"

	"github.com/brunolkatz/goprotos7/dbtool/db/db_models"
)

func (d *DB) CreateHeartbeat(ctx context.Context, hb *db_models.HeartbeatRegistration) error {
	return d.DbConn.WithContext(ctx).Model(db_models.HeartbeatRegistration{}).Create(hb).Error
}

func (d *DB) ListHeartbeats(ctx context.Context) ([]*db_models.HeartbeatRegistration, error) {
	var ret []*db_models.HeartbeatRegistration
	err := d.DbConn.WithContext(ctx).
		Model(db_models.HeartbeatRegistration{}).
		Order("db_number ASC, variable_name ASC").
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (d *DB) ListActiveHeartbeats(ctx context.Context) ([]*db_models.HeartbeatRegistration, error) {
	var ret []*db_models.HeartbeatRegistration
	err := d.DbConn.WithContext(ctx).
		Model(db_models.HeartbeatRegistration{}).
		Where("enabled = ? AND start_on_connect = ?", true, true).
		Find(&ret).Error
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (d *DB) GetHeartbeatByID(ctx context.Context, id int64) (*db_models.HeartbeatRegistration, error) {
	var hb db_models.HeartbeatRegistration
	err := d.DbConn.WithContext(ctx).
		Model(db_models.HeartbeatRegistration{}).
		Where("id = ?", id).
		First(&hb).Error
	if err != nil {
		return nil, err
	}
	return &hb, nil
}

func (d *DB) DeleteHeartbeatByID(ctx context.Context, id int64) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		err := d.DbConn.WithContext(ctx).
			Where("id = ?", id).
			Delete(db_models.HeartbeatRegistration{}).Error
		if err == nil {
			return nil
		}
		lastErr = err
		if !strings.Contains(strings.ToLower(err.Error()), "database is locked") {
			return err
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return lastErr
}

func (d *DB) ListEnabledHeartbeatAddresses(ctx context.Context) ([]string, error) {
	type row struct {
		Address string `gorm:"column:address"`
	}
	var rows []row
	err := d.DbConn.WithContext(ctx).
		Model(db_models.HeartbeatRegistration{}).
		Select("address").
		Where("enabled = ? AND start_on_connect = ?", true, true).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Address)
	}
	return out, nil
}

func (d *DB) UpdateHeartbeatRuntime(ctx context.Context, id int64, lastValue *bool, lastCheckAt, lastChangeAt *time.Time, isFailing bool, lastError string) error {
	return d.DbConn.WithContext(ctx).
		Model(db_models.HeartbeatRegistration{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_value":     lastValue,
			"last_check_at":  lastCheckAt,
			"last_change_at": lastChangeAt,
			"is_failing":     isFailing,
			"last_error":     lastError,
			"updated_at":     time.Now(),
		}).Error
}
