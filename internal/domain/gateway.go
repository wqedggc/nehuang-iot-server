package domain

import "time"

type Gateway struct {
	GatewayID         uint64    `json:"gateway_id,string"`
	GatewayExternalID string    `json:"gateway_external_id"`
	GatewayName       string    `json:"gateway_name"`
	AccessMode        string    `json:"access_mode"`
	IP                string    `json:"ip"`
	Port              int       `json:"port"`
	GatewaySN         string    `json:"gateway_sn"`
	OnlineStatus      uint8     `json:"online_status"`
	SourceType        uint8     `json:"source_type"`
	LastHeartbeatAtMS time.Time `json:"last_heartbeat_at_ms"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type GatewayAccessMode string

const (
	GatewayAccessModeShared    GatewayAccessMode = "shared"
	GatewayAccessModeExclusive GatewayAccessMode = "exclusive"
)

func NormalizeGatewayAccessMode(mode string) GatewayAccessMode {
	switch GatewayAccessMode(mode) {
	case GatewayAccessModeExclusive:
		return GatewayAccessModeExclusive
	default:
		return GatewayAccessModeShared
	}
}

func (m GatewayAccessMode) Label() string {
	switch m {
	case GatewayAccessModeExclusive:
		return "独享"
	default:
		return "共享"
	}
}
