// Package service 提供业务逻辑层的聚合查询与编排。
package service

import (
	"context"
	"fmt"
	"math"
	"time"
)

// DashboardRepository 定义 Dashboard 统计所需的数据访问接口。
// 每个统计指标一个独立方法，由 repository/mysql 实现。
type DashboardRepository interface {
	CountUsers(ctx context.Context) (totalUsers, adminCount, farmerCount, activeFarmerCount, disabledFarmerCount int64, err error)
	CountDeviceScale(ctx context.Context) (gatewayTotal, gatewayOnline, gatewayOffline, collectorNodeTotal, devicePointTotal, boundPointCount, unboundPointCount int64, err error)
	CountDevicePointStatus(ctx context.Context) (normalCount, abnormalCount, offlineCount int64, err error)
	LatestReportTime(ctx context.Context) (*time.Time, error)
	CountIngest(ctx context.Context, rangeCutoff time.Time) (rawEventCount, telemetryCount, registerCount, heartbeatCount, workCount, controlResultCount int64, err error)
	CountHeartbeatStats(ctx context.Context, rangeCutoff time.Time) (heartbeatTotal, heartbeatSuccess, heartbeatFailed int64, err error)
	CountControlStats(ctx context.Context, rangeCutoff time.Time) (commandCount, successCount, failedCount, timeoutCount int64, err error)
}

// DashboardService 聚合 Dashboard 所有统计查询。
type DashboardService struct {
	repo DashboardRepository
}

// NewDashboardService 创建 DashboardService。
func NewDashboardService(repo DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// validRanges 定义合法的 range 参数集合。
var validRanges = map[string]bool{
	"1h":  true,
	"6h":  true,
	"24h": true,
	"7d":  true,
}

// RangeDuration 将 range 参数解析为时间间隔。
// 非法 range 返回 error。
func RangeDuration(rangeStr string) (time.Duration, error) {
	switch rangeStr {
	case "1h":
		return 1 * time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid range: %s (allowed: 1h, 6h, 24h, 7d)", rangeStr)
	}
}

// ValidateRange 校验 range 参数是否合法。
func ValidateRange(rangeStr string) error {
	if !validRanges[rangeStr] {
		return fmt.Errorf("invalid range: %s (allowed: 1h, 6h, 24h, 7d)", rangeStr)
	}
	return nil
}

// DashboardData 是 Dashboard 接口的完整返回结构。
type DashboardData struct {
	Range       string       `json:"range"`
	GeneratedAt string       `json:"generated_at"`
	Users       UserStats    `json:"users"`
	Devices     DeviceStats  `json:"devices"`
	Ingest      IngestStats  `json:"ingest"`
	Control     ControlStats `json:"control"`
}

// UserStats 用户统计。
type UserStats struct {
	TotalUsers          int64 `json:"total_users"`
	AdminCount          int64 `json:"admin_count"`
	FarmerCount         int64 `json:"farmer_count"`
	ActiveFarmerCount   int64 `json:"active_farmer_count"`
	DisabledFarmerCount int64 `json:"disabled_farmer_count"`
}

// DeviceStats 设备统计。
type DeviceStats struct {
	GatewayTotal        int64  `json:"gateway_total"`
	GatewayOnline       int64  `json:"gateway_online"`
	GatewayOffline      int64  `json:"gateway_offline"`
	CollectorNodeTotal  int64  `json:"collector_node_total"`
	DevicePointTotal    int64  `json:"device_point_total"`
	BoundPointCount     int64  `json:"bound_point_count"`
	UnboundPointCount   int64  `json:"unbound_point_count"`
	NormalPointCount    int64  `json:"normal_point_count"`
	AbnormalPointCount  int64  `json:"abnormal_point_count"`
	OfflinePointCount   int64  `json:"offline_point_count"`
	LatestReportTime    string `json:"latest_report_time"`
}

// IngestStats 数据上报统计。
type IngestStats struct {
	RawEventCount       int64   `json:"raw_event_count"`
	TelemetryCount      int64   `json:"telemetry_count"`
	RegisterCount       int64   `json:"register_count"`
	HeartbeatCount      int64   `json:"heartbeat_count"`
	WorkCount           int64   `json:"work_count"`
	ControlResultCount  int64   `json:"control_result_count"`
	HeartbeatTotal      int64   `json:"heartbeat_total"`
	HeartbeatSuccess    int64   `json:"heartbeat_success"`
	HeartbeatFailed     int64   `json:"heartbeat_failed"`
	HeartbeatSuccessRate float64 `json:"heartbeat_success_rate"`
}

// ControlStats 控制统计。
type ControlStats struct {
	ControlCommandCount  int64   `json:"control_command_count"`
	ControlSuccessCount  int64   `json:"control_success_count"`
	ControlFailedCount   int64   `json:"control_failed_count"`
	ControlTimeoutCount  int64   `json:"control_timeout_count"`
	ControlSuccessRate   float64 `json:"control_success_rate"`
}

// Build 聚合所有统计查询并返回完整的 DashboardData。
// rangeStr 必须已通过 ValidateRange 校验。
func (s *DashboardService) Build(ctx context.Context, rangeStr string) (*DashboardData, error) {
	dur, err := RangeDuration(rangeStr)
	if err != nil {
		return nil, err
	}

	rangeCutoff := time.Now().Add(-dur)
	now := time.Now()

	// 用户统计（不依赖 range）
	totalUsers, adminCount, farmerCount, activeFarmerCount, disabledFarmerCount, err := s.repo.CountUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountUsers: %w", err)
	}

	// 设备规模（不依赖 range）
	gatewayTotal, gatewayOnline, gatewayOffline, collectorNodeTotal, devicePointTotal, boundPointCount, unboundPointCount, err := s.repo.CountDeviceScale(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountDeviceScale: %w", err)
	}

	// 功能点状态
	normalPointCount, abnormalPointCount, offlinePointCount, err := s.repo.CountDevicePointStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountDevicePointStatus: %w", err)
	}

	// 最近上报时间
	latestReportAt, err := s.repo.LatestReportTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("LatestReportTime: %w", err)
	}
	latestReportTime := ""
	if latestReportAt != nil {
		latestReportTime = latestReportAt.Format("2006-01-02T15:04:05.000-07:00")
	}

	// 数据上报统计
	rawEventCount, telemetryCount, registerCount, heartbeatCount, workCount, controlResultCount, err := s.repo.CountIngest(ctx, rangeCutoff)
	if err != nil {
		return nil, fmt.Errorf("CountIngest: %w", err)
	}

	// 心跳统计
	heartbeatTotal, heartbeatSuccess, heartbeatFailed, err := s.repo.CountHeartbeatStats(ctx, rangeCutoff)
	if err != nil {
		return nil, fmt.Errorf("CountHeartbeatStats: %w", err)
	}

	// 心跳成功率（百分比，保留两位小数）
	var heartbeatSuccessRate float64
	if heartbeatTotal > 0 {
		heartbeatSuccessRate = round2(float64(heartbeatSuccess) / float64(heartbeatTotal) * 100)
	}

	// 控制统计
	commandCount, successCount, failedCount, timeoutCount, err := s.repo.CountControlStats(ctx, rangeCutoff)
	if err != nil {
		return nil, fmt.Errorf("CountControlStats: %w", err)
	}

	// 控制成功率（百分比，保留两位小数）
	var controlSuccessRate float64
	if commandCount > 0 {
		controlSuccessRate = round2(float64(successCount) / float64(commandCount) * 100)
	}

	return &DashboardData{
		Range:       rangeStr,
		GeneratedAt: now.Format("2006-01-02T15:04:05.000-07:00"),
		Users: UserStats{
			TotalUsers:          totalUsers,
			AdminCount:          adminCount,
			FarmerCount:         farmerCount,
			ActiveFarmerCount:   activeFarmerCount,
			DisabledFarmerCount: disabledFarmerCount,
		},
		Devices: DeviceStats{
			GatewayTotal:        gatewayTotal,
			GatewayOnline:       gatewayOnline,
			GatewayOffline:      gatewayOffline,
			CollectorNodeTotal:  collectorNodeTotal,
			DevicePointTotal:    devicePointTotal,
			BoundPointCount:     boundPointCount,
			UnboundPointCount:   unboundPointCount,
			NormalPointCount:    normalPointCount,
			AbnormalPointCount:  abnormalPointCount,
			OfflinePointCount:   offlinePointCount,
			LatestReportTime:    latestReportTime,
		},
		Ingest: IngestStats{
			RawEventCount:        rawEventCount,
			TelemetryCount:       telemetryCount,
			RegisterCount:        registerCount,
			HeartbeatCount:       heartbeatCount,
			WorkCount:            workCount,
			ControlResultCount:   controlResultCount,
			HeartbeatTotal:       heartbeatTotal,
			HeartbeatSuccess:     heartbeatSuccess,
			HeartbeatFailed:      heartbeatFailed,
			HeartbeatSuccessRate: heartbeatSuccessRate,
		},
		Control: ControlStats{
			ControlCommandCount: commandCount,
			ControlSuccessCount: successCount,
			ControlFailedCount:  failedCount,
			ControlTimeoutCount: timeoutCount,
			ControlSuccessRate:  controlSuccessRate,
		},
	}, nil
}

// round2 保留两位小数。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
