package mysql

import (
	"context"
	"database/sql"
	"strings"

	"nehuang-iot-server/internal/domain"
)

// UpdateDevicePointConfig updates admin-maintained device point fields.
func (r *Repository) UpdateDevicePointConfig(ctx context.Context, pointID uint64, input domain.DevicePointUpdateInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var nodeID uint64
	if err := tx.QueryRowContext(ctx,
		`SELECT node_id FROM device_points WHERE point_id = ? LIMIT 1`, pointID,
	).Scan(&nodeID); err != nil {
		if err == sql.ErrNoRows {
			return sql.ErrNoRows
		}
		return err
	}

	pointSets, pointArgs := buildDevicePointConfigUpdate(input)
	if len(pointSets) > 0 {
		pointArgs = append(pointArgs, pointID)
		query := `UPDATE device_points SET ` + strings.Join(pointSets, ", ") + `, updated_at = NOW(3) WHERE point_id = ?`
		if _, err := tx.ExecContext(ctx, query, pointArgs...); err != nil {
			return err
		}
	}

	if input.ShedNo != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE collector_nodes SET shed_no = ?, updated_at = NOW(3) WHERE node_id = ?`,
			*input.ShedNo, nodeID,
		); err != nil {
			return err
		}
	}

	mapSets, mapArgs := buildRealtimeMapConfigUpdate(input)
	if len(mapSets) > 0 {
		mapArgs = append(mapArgs, pointID)
		query := `UPDATE device_realtime_maps SET ` + strings.Join(mapSets, ", ") + `, updated_at = NOW(3) WHERE point_id = ?`
		if _, err := tx.ExecContext(ctx, query, mapArgs...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func buildDevicePointConfigUpdate(input domain.DevicePointUpdateInput) ([]string, []any) {
	var sets []string
	var args []any

	if input.PointName != nil {
		sets = append(sets, "point_name = ?")
		args = append(args, *input.PointName)
	}
	if input.Unit != nil {
		sets = append(sets, "unit = ?")
		args = append(args, *input.Unit)
	}
	if input.BestMin != nil {
		sets = append(sets, "best_min = ?")
		args = append(args, *input.BestMin)
	}
	if input.BestMax != nil {
		sets = append(sets, "best_max = ?")
		args = append(args, *input.BestMax)
	}
	if input.RunMode != nil {
		sets = append(sets, "run_mode = ?")
		args = append(args, uint8(*input.RunMode))
	}
	if input.ControllableStatus != nil {
		sets = append(sets, "controllable_status = ?")
		args = append(args, *input.ControllableStatus)
	}
	if input.PollInterval != nil {
		sets = append(sets, "poll_interval = ?")
		args = append(args, *input.PollInterval)
	}
	if input.PollCmd != nil {
		sets = append(sets, "poll_cmd = ?")
		args = append(args, *input.PollCmd)
	}
	if input.PointStatus != nil {
		sets = append(sets, "point_status = ?")
		args = append(args, *input.PointStatus)
	}
	if input.Remark != nil {
		sets = append(sets, "remark = ?")
		args = append(args, *input.Remark)
	}

	return sets, args
}

func buildRealtimeMapConfigUpdate(input domain.DevicePointUpdateInput) ([]string, []any) {
	var sets []string
	var args []any

	if input.ShedNo != nil {
		sets = append(sets, "shed_no = ?")
		args = append(args, *input.ShedNo)
	}
	if input.Unit != nil {
		sets = append(sets, "unit = ?")
		args = append(args, *input.Unit)
	}
	if input.BestMin != nil {
		sets = append(sets, "best_min = ?")
		args = append(args, *input.BestMin)
	}
	if input.BestMax != nil {
		sets = append(sets, "best_max = ?")
		args = append(args, *input.BestMax)
	}
	if input.RunMode != nil {
		sets = append(sets, "run_mode = ?")
		args = append(args, uint8(*input.RunMode))
	}
	if input.PollInterval != nil {
		sets = append(sets, "poll_interval = ?")
		args = append(args, *input.PollInterval)
	}

	return sets, args
}
