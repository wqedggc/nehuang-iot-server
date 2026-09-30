package mysql

import (
	"context"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/idgen"
	"strings"
	"time"
)

// PersistMCU0TemperatureBatch commits raw-linked samples together. It deliberately
// does not infer/create devices or update commands from periodic observations.
func (r *Repository) PersistMCU0TemperatureBatch(ctx context.Context, b domain.MCU0TemperatureBatch) error {
	if b.RawEventID == 0 || b.PayloadHex == "" || len(b.Samples) == 0 {
		return fmt.Errorf("MCU0 incomplete batch")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status uint8
	var hex string
	if err = tx.QueryRowContext(ctx, `SELECT parse_status, payload_hex FROM raw_device_events WHERE raw_event_id = ? FOR UPDATE`, b.RawEventID).Scan(&status, &hex); err != nil {
		return err
	}
	if !strings.EqualFold(hex, b.PayloadHex) {
		return fmt.Errorf("MCU0 raw payload does not match batch")
	}
	if status == uint8(domain.ParseStatusParsed) {
		return tx.Commit()
	}
	if status != uint8(domain.ParseStatusPending) && status != uint8(domain.ParseStatusFailed) {
		return fmt.Errorf("MCU0 raw event not eligible for projection")
	}
	now := time.Now()
	for _, s := range b.Samples {
		if s.NodeID == nil || s.PointID == nil || s.RawEventID == nil || *s.RawEventID != b.RawEventID || s.RealValue == nil || s.SampledAtMS.IsZero() {
			return fmt.Errorf("MCU0 sample unresolved")
		}
		var point uint64
		if err = tx.QueryRowContext(ctx, `SELECT point_id FROM device_points WHERE point_id = ? AND node_id = ? AND gateway_id = ? AND frequency = ? AND device_external_id = ? AND function_type = 1 AND point_status = 1 FOR UPDATE`, *s.PointID, *s.NodeID, s.GatewayID, s.Frequency, s.DeviceExternalID).Scan(&point); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO sensor_data
 (sensor_id,gateway_id,realtime_id,node_id,point_id,frequency,device_external_id,sampled_at_ms,
 real_value,real_temp,function_type,unit,poll_interval,raw_event_id,created_at,updated_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, idgen.NextID(), s.GatewayID, nullableUint64(s.RealtimeID), *s.NodeID, *s.PointID, s.Frequency, s.DeviceExternalID, s.SampledAtMS, *s.RealValue, s.RealTemp, uint8(s.FunctionType), s.Unit, s.PollInterval, b.RawEventID, now, now)
		if err != nil {
			return err
		}
		if b.Realtime {
			_, err = tx.ExecContext(ctx, `UPDATE device_points SET real_value = ?, real_temp = ?, value_status = ?, last_report_at_ms = ?, last_raw_event_id = ?, updated_at = ? WHERE point_id = ? AND (last_report_at_ms IS NULL OR last_report_at_ms <= ?)`, *s.RealValue, s.RealTemp, uint8(domain.ValueStatusNormal), s.SampledAtMS, b.RawEventID, now, *s.PointID, s.SampledAtMS)
			if err != nil {
				return err
			}
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE raw_device_events SET parse_status = ?, parse_error = NULL, event_type = ?, event_count = ?, parsed_count = ? WHERE raw_event_id = ?`, uint8(domain.ParseStatusParsed), uint8(domain.RawEventWork), len(b.Samples), len(b.Samples), b.RawEventID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
