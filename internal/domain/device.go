// Package domain 定义涅凰智农 IoT 平台核心业务实体。
package domain

import "time"

// ─── 原始设备事件 ───

// RawDeviceEvent 对应 raw_device_events 表，记录所有设备事件的原始快照。
type RawDeviceEvent struct {
	RawEventID      uint64
	GatewayID       *uint64 // 解析出的云边设备ID，可能为空
	NodeID          *uint64 // 解析出的采集执行器节点ID，可能为空
	PointID         *uint64 // 解析出的功能点ID，可能为空
	GatewayName     *string // FP设备名
	EventType       RawEventType
	SourceType      SourceType
	RemoteIP        *string
	RemotePort      *int
	PayloadHex      *string // 原始十六进制
	PayloadSnapshot *string // 原始快照文本，禁止JSON类型
	ParseStatus     ParseStatus
	ParseError      *string
	ReceivedAtMS    time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// RawEventType 原始事件类型。
type RawEventType uint8

const (
	RawEventRegister       RawEventType = 1 // register：设备首次连接/注册
	RawEventHeartbeat      RawEventType = 2 // heartbeat：设备在线状态刷新
	RawEventWork           RawEventType = 3 // work：工作数据上报
	RawEventControlResult  RawEventType = 4 // control_result：控制命令执行结果回传
	RawEventJSONSnapshot   RawEventType = 5 // json_snapshot：JSON快照
)

// SourceType 事件来源。
type SourceType uint8

const (
	SourceTypeGateway SourceType = 1 // gateway：来自TCP Gateway
	SourceTypeAdapter SourceType = 2 // adapter：来自HTTP Adapter API
	SourceTypeManual  SourceType = 3 // manual：手动录入
)

// ParseStatus 解析状态。
type ParseStatus uint8

const (
	ParseStatusPending ParseStatus = 1 // pending：待解析
	ParseStatusParsed  ParseStatus = 2 // parsed：已解析
	ParseStatusFailed  ParseStatus = 3 // failed：解析失败
	ParseStatusIgnored ParseStatus = 4 // ignored：已忽略
)

// ─── 采集执行器节点 ───

// CollectorNode 对应 collector_nodes 表，表示一个采集执行器节点。
// 身份由 gateway_id + frequency + device_external_id 唯一确定。
type CollectorNode struct {
	NodeID           uint64
	GatewayID        uint64
	Frequency        int
	DeviceExternalID string
	NodeName         string
	ShedNo           string
	BaseName         string
	AreaName         string
	CollectorNo      *int
	NodeStatus       uint8  // 1=active 2=disabled 3=deleted
	OnlineStatus     uint8  // 0=离线 1=在线
	LastReportAtMS   *time.Time
	LastRawEventID   *uint64
	Remark           *string
	Extra            *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ─── 功能点 ───

// DevicePoint 对应 device_points 表，表示节点下的一个功能点。
// 身份由 node_id + function_type + channel_no 唯一确定。
type DevicePoint struct {
	PointID            uint64
	NodeID             uint64
	GatewayID          uint64
	Frequency          int
	DeviceExternalID   string
	FunctionType       FunctionType
	ChannelNo          int
	PointName          string
	Unit               string
	BestMin            *float64
	BestMax            *float64
	RealValue          *float64
	RealTemp           float64 // 甲方温度字段语义保留
	ValueStatus        ValueStatus
	RunMode            RunMode
	ControllableStatus uint8
	Rel1Status         uint8
	Rel2Status         uint8
	OperationState     OperationState
	PollInterval       int
	PollCmd            *string
	PointStatus        uint8 // 1=active 2=disabled 3=deleted
	LastReportAtMS     *time.Time
	LastRawEventID     *uint64
	Remark             *string
	Extra              *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// FunctionType 功能类型。
type FunctionType uint8

const (
	FuncTypeTemperature FunctionType = 1 // 温度
	FuncTypeSoil        FunctionType = 2 // 墒情
	FuncTypeLimit       FunctionType = 3 // 限位
	FuncTypeRelay       FunctionType = 4 // 继电器
	FuncTypePump        FunctionType = 5 // 水泵
	FuncTypeFan         FunctionType = 6 // 风机
)

// ValueStatus 数值状态。
type ValueStatus uint8

const (
	ValueStatusUnknown  ValueStatus = 0 // unknown
	ValueStatusNormal   ValueStatus = 1 // normal
	ValueStatusLow      ValueStatus = 2 // low
	ValueStatusHigh     ValueStatus = 3 // high
	ValueStatusOpen     ValueStatus = 4 // open
	ValueStatusClosed   ValueStatus = 5 // closed
	ValueStatusOffline  ValueStatus = 6 // offline
)

// RunMode 运行模式。
type RunMode uint8

const (
	RunModeAuto RunMode = 1 // auto：自动
	RunModeManu RunMode = 2 // manu：手动
)

// OperationState 操作状态。
type OperationState uint8

const (
	OpStateUnknown OperationState = 0 // unknown
	OpStateOpen    OperationState = 1 // open
	OpStateStop    OperationState = 2 // stop
	OpStateClose   OperationState = 3 // close
)

// ─── 传感器时序数据 ───

// SensorData 对应 sensor_data 表，记录功能点的历史采集数据。
type SensorData struct {
	SensorID         uint64
	GatewayID        uint64
	RealtimeID       *uint64
	NodeID           *uint64
	PointID          *uint64
	Frequency        int
	DeviceExternalID string
	SampledAtMS      time.Time
	RealValue        *float64
	RealTemp         float64
	FunctionType     FunctionType
	Unit             string
	PollInterval     int
	RawEventID       *uint64
	Extra            *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ─── 绑定关系 ───

// BindRelation 对应 bind_relations 表。
type BindRelation struct {
	BindID           uint64
	UserID           *uint64
	BindUserName     string
	GatewayID        uint64
	RealtimeID       *uint64
	NodeID           *uint64
	PointID          *uint64
	ShedNo           string
	Frequency        int
	DeviceExternalID string
	FunctionType     FunctionType
	BindStatus       BindStatus
	BoundAtMS        time.Time
	UnboundAtMS      *time.Time
	CreatedBy        *uint64
	Extra            *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// BindStatus 绑定状态。
type BindStatus uint8

const (
	BindStatusActive  BindStatus = 1 // active：已绑定
	BindStatusUnbound BindStatus = 2 // unbound：已解绑
	BindStatusDeleted BindStatus = 3 // deleted：已删除
)

// ─── 控制命令日志（简化视图，用于管理端控制日志列表） ───

// ControlLogSummary 控制日志列表摘要，供管理端控制日志页使用。
type ControlLogSummary struct {
	OpID         uint64    `json:"op_id"`
	GatewayID    uint64    `json:"gateway_id"`
	GatewayName  string    `json:"gateway_name"`
	BindUserName string    `json:"bind_usr_name"`
	Frequency    int       `json:"freq"`
	DevExternalID string   `json:"devid"`
	FunctionType uint8     `json:"func"`
	FunctionName string    `json:"func_name"`
	OpType       string    `json:"op_type"`
	RunMode      uint8     `json:"run_mode"`
	CmdStatus    uint8     `json:"cmd_status"`
	CmdStatusLabel string  `json:"cmd_status_label"`
	Result       string    `json:"result"`
	OpTime       time.Time `json:"op_time"`
}

// ─── C 端设备视图（聚合绑定+功能点） ───

// MiniDevice 是 C 端"我的设备"列表的聚合视图，将 bind_relations 与 device_points 合并。
type MiniDevice struct {
	PointID          uint64   `json:"point_id"`
	GatewayID        uint64   `json:"gateway_id"`
	GatewayName      string   `json:"gateway_name"`
	ShedNo           string   `json:"shed_no"`
	Frequency        int      `json:"freq"`
	DeviceExternalID string   `json:"devid"`
	FunctionType     uint8    `json:"func"`
	FunctionName     string   `json:"func_name"`
	RealValue        *float64 `json:"real_value"`
	Unit             string   `json:"unit"`
	ValueStatus      uint8    `json:"value_status"`
	ValueStatusLabel string   `json:"value_status_label"`
	RunMode          uint8    `json:"run_mode"`
	RunModeLabel     string   `json:"run_mode_label"`
	UpdatedAt        string   `json:"updated_at"`
}

// MiniDeviceDetail C 端设备详情（实时数据）。
type MiniDeviceDetail struct {
	PointID          uint64   `json:"point_id"`
	GatewayName      string   `json:"gateway_name"`
	ShedNo           string   `json:"shed_no"`
	Frequency        int      `json:"freq"`
	DeviceExternalID string   `json:"devid"`
	FunctionType     uint8    `json:"func"`
	FunctionName     string   `json:"func_name"`
	Unit             string   `json:"unit"`
	BestMin          *float64 `json:"best_min"`
	BestMax          *float64 `json:"best_max"`
	RealValue        *float64 `json:"real_value"`
	RealTemp         float64  `json:"real_temp"`
	ValueStatus      uint8    `json:"value_status"`
	ValueStatusLabel string   `json:"value_status_label"`
	RunMode          uint8    `json:"run_mode"`
	RunModeLabel     string   `json:"run_mode_label"`
	Rel1Status       uint8    `json:"rel1"`
	Rel2Status       uint8    `json:"rel2"`
	OperationState   uint8    `json:"operation_state"`
	PollInterval     int      `json:"poll_interval"`
	UpdatedAt        string   `json:"updated_at"`
}

// MiniHistoryPoint 历史曲线数据点。
type MiniHistoryPoint struct {
	Time     string  `json:"time"`
	RealTemp float64 `json:"real_temp"`
}

// MiniDeviceHistory C 端历史数据响应。
type MiniDeviceHistory struct {
	PointID   uint64             `json:"point_id"`
	FuncName  string             `json:"func_name"`
	Unit      string             `json:"unit"`
	BestMin   *float64           `json:"best_min"`
	BestMax   *float64           `json:"best_max"`
	Points    []MiniHistoryPoint `json:"points"`
}
