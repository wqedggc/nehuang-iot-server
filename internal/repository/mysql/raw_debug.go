package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"nehuang-iot-server/internal/domain"
	"nehuang-iot-server/internal/idgen"
	"strings"
	"time"
)

func (r *Repository) CreateRawDebugCommand(ctx context.Context, in domain.RawDebugCommandInput) (*domain.ControlCommand, error) {
	text, err := domain.ValidateRawDebugHex(in.CommandHex)
	if err != nil {
		return nil, err
	}
	if in.GatewayID == 0 || in.UserID == 0 || len(in.RequestID) != 32 || len(in.Remark) > 160 {
		return nil, fmt.Errorf("invalid raw debug identity/metadata")
	}
	for _, c := range in.RequestID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, fmt.Errorf("invalid request_id")
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var name, external string
	var online uint8
	if err = tx.QueryRowContext(ctx, `SELECT gateway_name, gateway_external_id, online_status FROM cloud_gateways WHERE gateway_id = ? FOR UPDATE`, in.GatewayID).Scan(&name, &external, &online); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrRawDebugConflict
		}
		return nil, err
	}
	if name != in.GatewayName || external == "" || online != 1 {
		return nil, domain.ErrRawDebugConflict
	}
	prefix := fmt.Sprintf(`{"source":"admin_raw_debug","request_id":"%s"`, in.RequestID)
	cmd := &domain.ControlCommand{GatewayID: in.GatewayID, CommandHex: text, RequestedBy: &in.UserID, CommandSource: domain.CommandSourceAdmin, CommandType: domain.CommandTypeCustomHex, RunMode: 2, SendOnce: true}
	var previousHex string
	err = tx.QueryRowContext(ctx, `SELECT command_id, command_hex, command_status FROM control_commands WHERE gateway_id = ? AND requested_by = ? AND extra LIKE ? ORDER BY requested_at_ms DESC LIMIT 1`, in.GatewayID, in.UserID, prefix+"%").Scan(&cmd.CommandID, &previousHex, &cmd.CommandStatus)
	if err == nil {
		if previousHex != text {
			return nil, domain.ErrRawDebugConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return cmd, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	var count int
	now := time.Now()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM control_commands WHERE gateway_id = ? AND extra LIKE ? AND requested_at_ms > ?`, in.GatewayID, `{"source":"admin_raw_debug"%`, now.Add(-2*time.Second)).Scan(&count); err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, domain.ErrRawDebugRateLimited
	}
	// Prefix layout is stable for the scoped request-id lookup; remainder is escaped.
	suffix, _ := json.Marshal(map[string]string{"remark": in.Remark, "send_policy": "once"})
	extra := prefix + "," + strings.TrimPrefix(string(suffix), "{")
	cmd.CommandID = idgen.NextID()
	cmd.CommandStatus = domain.CommandStatusCreated
	cmd.RequestedAtMS = now
	cmd.CreatedAt = now
	cmd.UpdatedAt = now
	cmd.Extra = &extra
	_, err = tx.ExecContext(ctx, `INSERT INTO control_commands (command_id,gateway_id,frequency,device_external_id,command_source,command_type,run_mode,command_hex,command_status,requested_by,requested_at_ms,extra,created_at,updated_at) VALUES (?,?,0,'',1,6,2,?,?,?,?,?,?,?)`, cmd.CommandID, in.GatewayID, text, uint8(cmd.CommandStatus), in.UserID, now, extra, now, now)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO op_records (op_id,gateway_id,user_id,bind_user_name,operator_role,frequency,device_external_id,run_mode,op_type,op_content,command_id,operated_at_ms,created_at,updated_at) VALUES (?,?,?,'',?,0,'',2,3,?,?,?,?,?)`, idgen.NextID(), in.GatewayID, in.UserID, uint8(in.Role), "管理员手动原始报文；"+in.Remark, cmd.CommandID, now, now, now)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// ClaimRawDebugCommand durably removes a manual command from pending BEFORE any
// physical write. A lost claim response means do not send; reconciliation is manual.
func (r *Repository) ClaimRawDebugCommand(ctx context.Context, id uint64, gatewayExternalID string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE control_commands cc JOIN cloud_gateways cg ON cg.gateway_id = cc.gateway_id SET cc.command_status = ?, cc.gateway_response = ?, cc.updated_at = ? WHERE cc.command_id = ? AND cg.gateway_external_id = ? AND cc.command_status = ? AND cc.extra LIKE ?`, uint8(domain.CommandStatusUnknown), "单次发送已领取，结果待确认；禁止自动重发", time.Now(), id, gatewayExternalID, uint8(domain.CommandStatusCreated), `{"source":"admin_raw_debug"%`)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return domain.ErrRawDebugConflict
	}
	return nil
}
