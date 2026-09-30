package domain

import "time"

type CommandSource uint8

const (
	CommandSourceAdmin  CommandSource = 1
	CommandSourceMini   CommandSource = 2
	CommandSourceAuto   CommandSource = 3
	CommandSourceSystem CommandSource = 4
)

func (s CommandSource) Label() string {
	switch s {
	case CommandSourceAdmin:
		return "Admin"
	case CommandSourceMini:
		return "Mini"
	case CommandSourceAuto:
		return "自动"
	case CommandSourceSystem:
		return "系统"
	default:
		return "未知"
	}
}

type CommandType uint8

const (
	CommandTypeRelayOpen  CommandType = 1
	CommandTypeRelayClose CommandType = 2
	CommandTypeRelayStop  CommandType = 3
	CommandTypePoll       CommandType = 4
	CommandTypeReadStatus CommandType = 5
	CommandTypeCustomHex  CommandType = 6
)

type CommandStatus uint8

const (
	CommandStatusCreated   CommandStatus = 1
	CommandStatusSent      CommandStatus = 2
	CommandStatusAck       CommandStatus = 3
	CommandStatusSucceeded CommandStatus = 4
	CommandStatusFailed    CommandStatus = 5
	CommandStatusTimeout   CommandStatus = 6
	CommandStatusUnknown   CommandStatus = 7 // attempt claimed; outcome unknown, never auto-requeued
)

func (s CommandStatus) Label() string {
	switch s {
	case CommandStatusCreated:
		return "pending"
	case CommandStatusSent:
		return "sent"
	case CommandStatusAck:
		return "ack"
	case CommandStatusSucceeded:
		return "succeeded"
	case CommandStatusFailed:
		return "failed"
	case CommandStatusTimeout:
		return "timeout"
	case CommandStatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

type ControlCommand struct {
	CommandID        uint64        `json:"command_id,string"`
	GatewayID        uint64        `json:"gateway_id,string"`
	RealtimeID       *uint64       `json:"realtime_id,string,omitempty"`
	NodeID           *uint64       `json:"node_id,string,omitempty"`
	PointID          *uint64       `json:"point_id,string,omitempty"`
	Frequency        int           `json:"frequency"`
	DeviceExternalID string        `json:"device_external_id"`
	CommandSource    CommandSource `json:"command_source"`
	CommandType      CommandType   `json:"command_type"`
	RunMode          RunMode       `json:"run_mode"`
	Rel1TargetStatus *uint8        `json:"rel1_target_status,omitempty"`
	Rel2TargetStatus *uint8        `json:"rel2_target_status,omitempty"`
	CommandHex       string        `json:"command_hex"`
	CommandStatus    CommandStatus `json:"command_status"`
	SendOnce         bool          `json:"send_once,omitempty"`
	RequestedBy      *uint64       `json:"requested_by,string,omitempty"`
	RequestedAtMS    time.Time     `json:"requested_at_ms"`
	SentAtMS         *time.Time    `json:"sent_at_ms,omitempty"`
	FinishedAtMS     *time.Time    `json:"finished_at_ms,omitempty"`
	GatewayResponse  *string       `json:"gateway_response,omitempty"`
	ErrorMessage     *string       `json:"error_message,omitempty"`
	Extra            *string       `json:"extra,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type CreateControlCommandInput struct {
	PointID          uint64
	RequestedBy      *uint64
	CommandSource    CommandSource
	CommandType      CommandType
	CommandHex       string
	Rel1TargetStatus *uint8
	Rel2TargetStatus *uint8
	Extra            *string
}

type AdapterCommandResultInput struct {
	CommandID       uint64
	Status          CommandStatus
	GatewayResponse *string
	ErrorMessage    *string
	ResultHex       *string
	ResultSnapshot  *string
}

// IsTerminal reports whether automatic result processing has completed.
func (s CommandStatus) IsTerminal() bool {
	return s == CommandStatusSucceeded || s == CommandStatusFailed || s == CommandStatusTimeout
}

// CanAdvanceResult keeps duplicate and late responses from rewriting command state.
// Responses remain audit records even when they do not advance this state.
func (s CommandStatus) CanAdvanceResult(next CommandStatus) bool {
	if s.IsTerminal() || s == next {
		return false
	}
	if s != CommandStatusCreated && s != CommandStatusSent && s != CommandStatusAck && s != CommandStatusUnknown {
		return false
	}
	return next == CommandStatusAck || next.IsTerminal()
}
