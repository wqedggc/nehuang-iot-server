// Package domain 定义涅凰智农 IoT 平台核心业务实体。
package domain

import (
	"errors"
	"time"
)

var ErrBindingConflict = errors.New("binding conflict")

// OpType 操作记录类型。
type OpType uint8

const (
	OpTypeUnknown OpType = 0
	OpTypeBind    OpType = 1
	OpTypeUnbind  OpType = 2
	OpTypeControl OpType = 3
	OpTypeConfig  OpType = 4
)

func (t OpType) Label() string {
	switch t {
	case OpTypeBind:
		return "绑定"
	case OpTypeUnbind:
		return "解绑"
	case OpTypeControl:
		return "控制"
	case OpTypeConfig:
		return "配置"
	default:
		return "未知"
	}
}

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
	MessageID       *string // 消息唯一ID（从 payload 解析）
	EventCount      int     // events[] 数组长度
	ParsedCount     int     // 成功解析的 events 数量
	ReceivedAtMS    time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// RawEventTypeName returns the Chinese label for a RawEventType.
func (t RawEventType) RawEventTypeName() string {
	switch t {
	case RawEventRegister:
		return "注册"
	case RawEventHeartbeat:
		return "心跳"
	case RawEventWork:
		return "工作数据"
	case RawEventControlResult:
		return "控制结果"
	case RawEventJSONSnapshot:
		return "JSON快照"
	case RawEventAlarm:
		return "告警"
	case RawEventUnknown:
		return "未知包"
	default:
		return "未知"
	}
}

// RawEventTypeCSS returns a CSS class suffix for event type badges.
func (t RawEventType) RawEventTypeCSS() string {
	switch t {
	case RawEventRegister:
		return "info"
	case RawEventHeartbeat:
		return "muted"
	case RawEventWork:
		return "success"
	case RawEventControlResult:
		return "warning"
	case RawEventJSONSnapshot:
		return "primary"
	case RawEventAlarm:
		return "danger"
	case RawEventUnknown:
		return "muted"
	default:
		return ""
	}
}

// RawEventType 原始事件类型。
type RawEventType uint8

const (
	RawEventRegister      RawEventType = 1 // register：设备首次连接/注册
	RawEventHeartbeat     RawEventType = 2 // heartbeat：设备在线状态刷新
	RawEventWork          RawEventType = 3 // work：工作数据上报
	RawEventControlResult RawEventType = 4 // control_result：控制命令执行结果回传
	RawEventJSONSnapshot  RawEventType = 5 // json_snapshot：JSON快照
	RawEventAlarm         RawEventType = 6 // alarm：告警原始包
	RawEventUnknown       RawEventType = 7 // unknown：未知原始包
)

// SourceType 事件来源。
type SourceType uint8

const (
	SourceTypeGateway SourceType = 1 // gateway：来自TCP Gateway
	SourceTypeAdapter SourceType = 2 // adapter：来自HTTP Adapter API
	SourceTypeManual  SourceType = 3 // manual：手动录入
	SourceTypeMock    SourceType = 4 // mock：来自 mock_device_events.sh 等模拟上报脚本
)

func (s SourceType) SourceTypeName() string {
	switch s {
	case SourceTypeGateway:
		return "gateway"
	case SourceTypeAdapter:
		return "adapter"
	case SourceTypeManual:
		return "manual"
	case SourceTypeMock:
		return "mock_script"
	default:
		return "unknown"
	}
}

// ParseStatus 解析状态。
type ParseStatus uint8

const (
	ParseStatusPending ParseStatus = 1 // pending：待解析
	ParseStatusParsed  ParseStatus = 2 // parsed：全部解析成功
	ParseStatusFailed  ParseStatus = 3 // failed：解析失败
	ParseStatusPartial ParseStatus = 4 // partial：部分解析（新增）
	ParseStatusIgnored ParseStatus = 5 // ignored：手动忽略
)

// ParseStatusName returns the Chinese label for a ParseStatus.
func (s ParseStatus) ParseStatusName() string {
	switch s {
	case ParseStatusPending:
		return "待解析"
	case ParseStatusParsed:
		return "已解析"
	case ParseStatusFailed:
		return "解析失败"
	case ParseStatusPartial:
		return "部分解析"
	case ParseStatusIgnored:
		return "已忽略"
	default:
		return "未知"
	}
}

// ParseStatusCSS returns a CSS class suffix for status badges.
func (s ParseStatus) ParseStatusCSS() string {
	switch s {
	case ParseStatusPending:
		return "pending"
	case ParseStatusParsed:
		return "success"
	case ParseStatusFailed:
		return "danger"
	case ParseStatusPartial:
		return "warning"
	case ParseStatusIgnored:
		return "muted"
	default:
		return ""
	}
}

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
	NodeStatus       uint8 // 1=active 2=disabled 3=deleted
	OnlineStatus     uint8 // 0=离线 1=在线
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
	RealTemp           float64 // 既有温度字段语义保留
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

// DevicePointUpdateInput is the admin-side patch payload for device point config.
type DevicePointUpdateInput struct {
	PointName          *string
	ShedNo             *string
	Unit               *string
	BestMin            *float64
	BestMax            *float64
	RunMode            *RunMode
	ControllableStatus *uint8
	PollInterval       *int
	PollCmd            *string
	PointStatus        *uint8
	Remark             *string
}

// GatewayDeviceSeedInput is an admin-only 联调 helper for creating a collector
// node and several device points under a registered gateway before the hardware
// work package parser is finalized.
type GatewayDeviceSeedInput struct {
	Frequency        int                           `json:"frequency"`
	DeviceExternalID string                        `json:"device_external_id"`
	NodeName         string                        `json:"node_name"`
	ShedNo           string                        `json:"shed_no"`
	Remark           string                        `json:"remark"`
	Points           []GatewayDeviceSeedPointInput `json:"points"`
}

type GatewayDeviceSeedPointInput struct {
	FunctionType       FunctionType `json:"function_type"`
	ChannelNo          int          `json:"channel_no"`
	PointName          string       `json:"point_name"`
	Unit               string       `json:"unit"`
	BestMin            *float64     `json:"best_min"`
	BestMax            *float64     `json:"best_max"`
	RealValue          *float64     `json:"real_value"`
	ValueStatus        ValueStatus  `json:"value_status"`
	RunMode            RunMode      `json:"run_mode"`
	ControllableStatus uint8        `json:"controllable_status"`
	PollInterval       int          `json:"poll_interval"`
	PollCmd            string       `json:"poll_cmd"`
	PointStatus        uint8        `json:"point_status"`
	Remark             string       `json:"remark"`
}

type GatewayDeviceSeedResult struct {
	GatewayID     uint64   `json:"gateway_id,string"`
	NodeID        uint64   `json:"node_id,string"`
	PointIDs      []string `json:"point_ids"`
	CreatedPoints int      `json:"created_points"`
	UpdatedPoints int      `json:"updated_points"`
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

// FunctionTypeName returns the Chinese name for a FunctionType.
func (ft FunctionType) FunctionTypeName() string {
	switch ft {
	case FuncTypeTemperature:
		return "温度"
	case FuncTypeSoil:
		return "墒情"
	case FuncTypeLimit:
		return "限位"
	case FuncTypeRelay:
		return "继电器"
	case FuncTypePump:
		return "水泵"
	case FuncTypeFan:
		return "风机"
	default:
		return "未知"
	}
}

// ValueStatus 数值状态。
type ValueStatus uint8

const (
	ValueStatusUnknown ValueStatus = 0 // unknown
	ValueStatusNormal  ValueStatus = 1 // normal
	ValueStatusLow     ValueStatus = 2 // low
	ValueStatusHigh    ValueStatus = 3 // high
	ValueStatusOpen    ValueStatus = 4 // open
	ValueStatusClosed  ValueStatus = 5 // closed
	ValueStatusOffline ValueStatus = 6 // offline
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
	BindID            uint64                   `json:"bind_id,string"`
	UserID            *uint64                  `json:"user_id,string"`
	BindUserName      string                   `json:"bind_user_name"`
	BindingType       string                   `json:"binding_type"`
	GatewayID         uint64                   `json:"gateway_id,string"`
	GatewayName       string                   `json:"gateway_name"`
	GatewayExternalID string                   `json:"gateway_external_id"`
	RealtimeID        *uint64                  `json:"realtime_id,string"`
	NodeID            *uint64                  `json:"node_id,string"`
	PointID           *uint64                  `json:"point_id,string"`
	ShedNo            string                   `json:"shed_no"`
	Frequency         int                      `json:"frequency"`
	DeviceExternalID  string                   `json:"device_external_id"`
	FunctionType      FunctionType             `json:"function_type"`
	Collector         *BindingCollectorSummary `json:"collector,omitempty"`
	BindStatus        BindStatus               `json:"bind_status"`
	BoundAtMS         time.Time                `json:"bound_at_ms"`
	UnboundAtMS       *time.Time               `json:"unbound_at_ms"`
	CreatedBy         *uint64                  `json:"created_by,string"`
	Extra             *string                  `json:"extra"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

// BindingCollectorSummary 是绑定列表中点位摘要的响应视图。
type BindingCollectorSummary struct {
	PointID          *uint64      `json:"point_id,string,omitempty"`
	NodeID           uint64       `json:"node_id,string"`
	NodeName         string       `json:"node_name,omitempty"`
	PointName        string       `json:"point_name"`
	FunctionType     FunctionType `json:"function_type"`
	FunctionName     string       `json:"func_name"`
	ChannelNo        int          `json:"channel_no"`
	ShedNo           string       `json:"shed_no"`
	Frequency        int          `json:"freq"`
	DeviceExternalID string       `json:"devid"`
}

// BindingType 绑定类型。
type BindingType string

const (
	BindingTypeGateway   BindingType = "gateway"
	BindingTypeCollector BindingType = "collector"
	BindingTypePoint     BindingType = "point"
)

// GatewayBindCandidate 是管理端网关级绑定弹窗的候选项。
type GatewayBindCandidate struct {
	GatewayID         uint64 `json:"gateway_id,string"`
	GatewayName       string `json:"gateway_name"`
	GatewayExternalID string `json:"gateway_external_id"`
	AccessMode        string `json:"access_mode"`
	AccessModeLabel   string `json:"access_mode_label"`
	OnlineStatus      uint8  `json:"online_status"`
	LastHeartbeatAtMS string `json:"last_heartbeat_at_ms"`
}

// BindStatus 绑定状态。
type BindStatus uint8

const (
	BindStatusActive  BindStatus = 1 // active：已绑定
	BindStatusUnbound BindStatus = 2 // unbound：已解绑
	BindStatusDeleted BindStatus = 3 // deleted：已删除
)

// OperationRecord 对应 op_records 表，用于后台操作审计。
type OperationRecord struct {
	OpID             uint64    `json:"op_id,string"`
	GatewayID        uint64    `json:"gateway_id,string"`
	GatewayName      string    `json:"gateway_name,omitempty"`
	RealtimeID       *uint64   `json:"realtime_id,string,omitempty"`
	NodeID           *uint64   `json:"node_id,string,omitempty"`
	PointID          *uint64   `json:"point_id,string,omitempty"`
	UserID           *uint64   `json:"user_id,string,omitempty"`
	OperatorName     string    `json:"operator_name,omitempty"`
	BindUserName     string    `json:"bind_user_name"`
	OperatorRole     uint8     `json:"operator_role"`
	Frequency        int       `json:"frequency"`
	DeviceExternalID string    `json:"device_external_id"`
	RunMode          uint8     `json:"run_mode"`
	Rel1Status       uint8     `json:"rel1_status"`
	Rel2Status       uint8     `json:"rel2_status"`
	OpType           OpType    `json:"op_type"`
	OpTypeLabel      string    `json:"op_type_label"`
	OpContent        string    `json:"op_content,omitempty"`
	CommandID        *uint64   `json:"command_id,string,omitempty"`
	OperatedAtMS     time.Time `json:"operated_at_ms"`
	CreatedAt        time.Time `json:"created_at"`
}

// OperationLogQuery is query args for admin operation logs list.
type OperationLogQuery struct {
	Page      int
	Size      int
	Paginated bool
	Start     *string `json:"start"`
	End       *string `json:"end"`
	Keyword   *string `json:"keyword"`
	OpType    *uint8  `json:"op_type"`
}

// ─── 控制命令日志（简化视图，用于管理端控制日志列表） ───

// ControlLogSummary 控制日志列表摘要，供管理端控制日志页使用。
type ControlLogSummary struct {
	CommandID       uint64           `json:"command_id,string,omitempty"`
	OpID            uint64           `json:"op_id,string"`
	GatewayID       uint64           `json:"gateway_id,string"`
	GatewayName     string           `json:"gateway_name"`
	NodeID          uint64           `json:"node_id,string,omitempty"`
	PointID         uint64           `json:"point_id,string,omitempty"`
	BindUserName    string           `json:"bind_usr_name"`
	RequestedBy     uint64           `json:"requested_by,string,omitempty"`
	CommandSource   uint8            `json:"command_source"`
	SourceLabel     string           `json:"command_source_label"`
	Frequency       int              `json:"freq"`
	DevExternalID   string           `json:"devid"`
	FunctionType    uint8            `json:"func"`
	FunctionName    string           `json:"func_name"`
	OpType          string           `json:"op_type"`
	RunMode         uint8            `json:"run_mode"`
	CmdStatus       uint8            `json:"cmd_status"`
	CmdStatusLabel  string           `json:"cmd_status_label"`
	CommandHex      string           `json:"command_hex"`
	Result          string           `json:"result"`
	GatewayResponse string           `json:"gateway_response"`
	ErrorMessage    string           `json:"error_message"`
	OpTime          time.Time        `json:"op_time"`
	SentAt          *time.Time       `json:"sent_at_ms,omitempty"`
	FinishedAt      *time.Time       `json:"finished_at_ms,omitempty"`
	ProtocolSummary *ProtocolSummary `json:"protocol_summary,omitempty"`
}

// ControlLogQuery is query args for admin control logs list.
type ControlLogQuery struct {
	Page      int
	Size      int
	Paginated bool
	Start     *string `json:"start"`
	End       *string `json:"end"`
	Keyword   *string `json:"keyword"`
	CmdStatus *uint8  `json:"cmd_status"`
	Source    *uint8  `json:"source"`
}

// ─── C 端设备视图（聚合绑定+功能点） ───

// MiniDevice 是 C 端"我的设备"列表的聚合视图，将 bind_relations 与 device_points 合并。
type MiniDevice struct {
	PointID                uint64   `json:"point_id,string"`
	NodeID                 uint64   `json:"node_id,string"`
	NodeName               string   `json:"node_name"`
	GatewayID              uint64   `json:"gateway_id,string"`
	GatewayName            string   `json:"gateway_name"`
	GatewayAccessMode      string   `json:"gateway_access_mode"`
	GatewayAccessModeLabel string   `json:"gateway_access_mode_label"`
	ShedNo                 string   `json:"shed_no"`
	Frequency              int      `json:"freq"`
	DeviceExternalID       string   `json:"devid"`
	FunctionType           uint8    `json:"func"`
	FunctionName           string   `json:"func_name"`
	RealValue              *float64 `json:"real_value"`
	Unit                   string   `json:"unit"`
	ValueStatus            uint8    `json:"value_status"`
	ValueStatusLabel       string   `json:"value_status_label"`
	RunMode                uint8    `json:"run_mode"`
	RunModeLabel           string   `json:"run_mode_label"`
	UpdatedAt              string   `json:"updated_at"`
}

// MiniDeviceDetail C 端设备详情（实时数据）。
type MiniDeviceDetail struct {
	PointID                uint64   `json:"point_id,string"`
	NodeID                 uint64   `json:"node_id,string"`
	NodeName               string   `json:"node_name"`
	GatewayName            string   `json:"gateway_name"`
	GatewayAccessMode      string   `json:"gateway_access_mode"`
	GatewayAccessModeLabel string   `json:"gateway_access_mode_label"`
	ShedNo                 string   `json:"shed_no"`
	Frequency              int      `json:"freq"`
	DeviceExternalID       string   `json:"devid"`
	FunctionType           uint8    `json:"func"`
	FunctionName           string   `json:"func_name"`
	Unit                   string   `json:"unit"`
	BestMin                *float64 `json:"best_min"`
	BestMax                *float64 `json:"best_max"`
	RealValue              *float64 `json:"real_value"`
	RealTemp               float64  `json:"real_temp"`
	ValueStatus            uint8    `json:"value_status"`
	ValueStatusLabel       string   `json:"value_status_label"`
	RunMode                uint8    `json:"run_mode"`
	RunModeLabel           string   `json:"run_mode_label"`
	Rel1Status             uint8    `json:"rel1"`
	Rel2Status             uint8    `json:"rel2"`
	OperationState         uint8    `json:"operation_state"`
	PollInterval           int      `json:"poll_interval"`
	UpdatedAt              string   `json:"updated_at"`
}

// MiniHistoryPoint 历史曲线数据点。
type MiniHistoryPoint struct {
	Time     string  `json:"time"`
	RealTemp float64 `json:"real_temp"`
}

// MiniDeviceHistory C 端历史数据响应。
type MiniDeviceHistory struct {
	PointID  uint64             `json:"point_id,string"`
	FuncName string             `json:"func_name"`
	Unit     string             `json:"unit"`
	BestMin  *float64           `json:"best_min"`
	BestMax  *float64           `json:"best_max"`
	Points   []MiniHistoryPoint `json:"points"`
}

// ─── 设备树（管理端 device-tree 接口） ───

// DeviceTreeNode 是设备树的顶层节点，对应一个 cloud_gateway。
type DeviceTreeNode struct {
	GatewayID       uint64                `json:"gateway_id,string"`
	GatewayName     string                `json:"gateway_name"`
	AccessMode      string                `json:"access_mode"`
	AccessModeLabel string                `json:"access_mode_label"`
	Status          uint8                 `json:"status"`
	Collectors      []DeviceTreeCollector `json:"collectors"`
}

// DeviceTreeCollector 是设备树的第二层，对应一个 collector_node。
type DeviceTreeCollector struct {
	NodeID           uint64            `json:"node_id,string"`
	NodeName         string            `json:"node_name"`
	ShedNo           string            `json:"shed_no"`
	DeviceExternalID string            `json:"device_external_id"`
	Frequency        int               `json:"frequency"`
	IsBound          bool              `json:"is_bound"`
	BoundUserID      *uint64           `json:"bound_user_id,string,omitempty"`
	Points           []DeviceTreePoint `json:"points"`
}

// DeviceTreePoint 是设备树的第三层，对应一个 device_point。
type DeviceTreePoint struct {
	PointID            uint64   `json:"point_id,string"`
	FunctionType       uint8    `json:"function_type"`
	FunctionName       string   `json:"function_name"`
	ChannelNo          int      `json:"channel_no"`
	PointName          string   `json:"point_name"`
	ShedNo             string   `json:"shed_no"`
	Unit               string   `json:"unit"`
	BestMin            *float64 `json:"best_min"`
	BestMax            *float64 `json:"best_max"`
	RunMode            uint8    `json:"run_mode"`
	ControllableStatus uint8    `json:"controllable_status"`
	PollInterval       int      `json:"poll_interval"`
	PollCmd            string   `json:"poll_cmd"`
	PointStatus        uint8    `json:"point_status"`
	Remark             string   `json:"remark"`
	IsBound            bool     `json:"is_bound"`
	BoundUserID        *uint64  `json:"bound_user_id,string"`
}

// DevicePointSearchResult 是管理端设备点位搜索的结果视图。
type DevicePointSearchResult struct {
	PointID          uint64       `json:"point_id,string"`
	GatewayID        uint64       `json:"gateway_id,string"`
	GatewayName      string       `json:"gateway_name"`
	NodeID           uint64       `json:"node_id,string"`
	Frequency        int          `json:"frequency"`
	DeviceExternalID string       `json:"device_external_id"`
	FunctionType     FunctionType `json:"function_type"`
	FunctionName     string       `json:"function_name"`
	ChannelNo        int          `json:"channel_no"`
	PointName        string       `json:"point_name"`
	Unit             string       `json:"unit"`
	ShedNo           string       `json:"shed_no"`
	IsBound          bool         `json:"is_bound"`
	BindUserID       *uint64      `json:"bind_user_id,string,omitempty"`
	BindUserName     string       `json:"bind_user_name,omitempty"`
}

// ─── Payload 排查页面 ───

// PayloadEventQuery 是 payload 事件列表查询参数。
type PayloadEventQuery struct {
	Page        int     `json:"page"`
	Size        int     `json:"size"`
	SourceType  *uint8  `json:"source_type"`
	EventType   *uint8  `json:"event_type"`
	ParseStatus *uint8  `json:"parse_status"`
	Start       *string `json:"start"`
	End         *string `json:"end"`
	Keyword     *string `json:"keyword"`
}

// PayloadEventListItem 是 payload 事件列表项。
type PayloadEventListItem struct {
	RawEventID      uint64 `json:"raw_event_id,string"`
	GatewayName     string `json:"gateway_name"`
	SourceType      uint8  `json:"source_type"`
	SourceTypeName  string `json:"source_type_name"`
	IsMock          bool   `json:"is_mock"`
	EventType       uint8  `json:"event_type"`
	EventTypeName   string `json:"event_type_name"`
	ParseStatus     uint8  `json:"parse_status"`
	ParseStatusName string `json:"parse_status_name"`
	ParseError      string `json:"parse_error"`
	MessageID       string `json:"message_id"`
	EventCount      int    `json:"event_count"`
	ParsedCount     int    `json:"parsed_count"`
	RemoteIP        string `json:"remote_ip"`
	RemotePort      int    `json:"remote_port"`
	ReceivedAtMS    string `json:"received_at_ms"`
	PayloadLen      int    `json:"payload_len"`
	ProtocolLabel   string `json:"protocol_label,omitempty"`
	DeviceIdentity  string `json:"device_identity,omitempty"`
	PayloadHex      string `json:"-"`
}

// ProtocolSummary is a display-only interpretation of raw bytes for debugging.
// It does not mean the raw event has been projected into business tables.
type ProtocolSummary struct {
	Kind        string                 `json:"kind"`
	Label       string                 `json:"label"`
	Description string                 `json:"description"`
	Status      string                 `json:"status,omitempty"`
	ByteLength  int                    `json:"byte_length,omitempty"`
	RecordCount int                    `json:"record_count,omitempty"`
	ASCII       string                 `json:"ascii,omitempty"`
	CRCValid    *bool                  `json:"crc_valid,omitempty"`
	Fields      []ProtocolSummaryField `json:"fields,omitempty"`
	Diagnostics []ProtocolDiagnostic   `json:"diagnostics,omitempty"`
}

// ProtocolSummaryField is one decoded byte range for Admin raw payload detail.
type ProtocolSummaryField struct {
	Name      string               `json:"name"`
	Label     string               `json:"label"`
	Value     string               `json:"value"`
	ByteStart int                  `json:"byte_start"`
	ByteEnd   int                  `json:"byte_end"`
	RawHex    string               `json:"raw_hex,omitempty"`
	Status    string               `json:"status,omitempty"`
	Note      string               `json:"note,omitempty"`
	Bits      []ProtocolSummaryBit `json:"bits,omitempty"`
}

// ProtocolSummaryBit explains one bit in a bitmap field such as MCU1 Tag.
type ProtocolSummaryBit struct {
	Position int    `json:"position"`
	BitIndex int    `json:"bit_index"`
	Symbol   string `json:"symbol"`
	Value    uint8  `json:"value"`
	Meaning  string `json:"meaning"`
	Status   string `json:"status,omitempty"`
}

// ProtocolDiagnostic explains why a frame is partial or invalid without
// changing the raw payload or claiming that it reached business tables.
type ProtocolDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// PayloadEventDetail 是 payload 事件详情。
type PayloadEventDetail struct {
	RawEventID      uint64           `json:"raw_event_id,string"`
	GatewayID       *uint64          `json:"gateway_id,string,omitempty"`
	NodeID          *uint64          `json:"node_id,string,omitempty"`
	PointID         *uint64          `json:"point_id,string,omitempty"`
	GatewayName     string           `json:"gateway_name"`
	SourceType      uint8            `json:"source_type"`
	SourceTypeName  string           `json:"source_type_name"`
	IsMock          bool             `json:"is_mock"`
	EventType       uint8            `json:"event_type"`
	EventTypeName   string           `json:"event_type_name"`
	ParseStatus     uint8            `json:"parse_status"`
	ParseStatusName string           `json:"parse_status_name"`
	ParseError      string           `json:"parse_error"`
	MessageID       string           `json:"message_id"`
	EventCount      int              `json:"event_count"`
	ParsedCount     int              `json:"parsed_count"`
	RemoteIP        string           `json:"remote_ip"`
	RemotePort      int              `json:"remote_port"`
	PayloadHex      string           `json:"payload_hex"`
	PayloadByteLen  int              `json:"payload_byte_len"`
	ProtocolSummary *ProtocolSummary `json:"protocol_summary,omitempty"`
	PayloadSnapshot string           `json:"payload_snapshot"`
	ReceivedAtMS    string           `json:"received_at_ms"`
	CreatedAt       string           `json:"created_at"`
}
