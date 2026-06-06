package contracts

// ApiCallReq 对齐  ApiCallReq。
// 这是外部 API 外呼在 API、CTI、ESL 之间透传的最小兼容请求。
type ApiCallReq struct {
	UserID int    `json:"userId"`
	Extra  string `json:"extra,omitempty"`
	Callee string `json:"callee"`
}

// BatchCallReq 对齐  BatchCallReq。
// CTI 调度器下发单个批量号码时，ESL 侧应直接使用这些元数据构造起呼计划。
type BatchCallReq struct {
	UserID         int            `json:"userId"`
	BatchTaskID    int            `json:"batchTaskId"`
	CallTaskState  BatchTaskState `json:"callTaskState,omitempty"`
	BatchCallTelID int            `json:"batchCallTelId"`
	Phone          string         `json:"phone"`
	MerchantID     int            `json:"merchantId"`
	SeatNumber     string         `json:"seatNumber,omitempty"`
	UserName       string         `json:"userName,omitempty"`
	Extension      string         `json:"extension"`
	ExtensionID    int            `json:"extensionId,omitempty"`
	AIFlag         bool           `json:"aiFlag,omitempty"`
	Push           bool           `json:"push"`
	Extra          string         `json:"extra,omitempty"`
	CallMode       int            `json:"callMode,omitempty"`
	CallRatio      float64        `json:"callRatio,omitempty"`
	QueueEnable    bool           `json:"queueEnable,omitempty"`
}

type BatchTaskState string

const (
	BatchTaskNotStarted BatchTaskState = "NOT_STARTED"
	BatchTaskRunning    BatchTaskState = "RUNNING"
	BatchTaskPaused     BatchTaskState = "PAUSED"
	BatchTaskCompleted  BatchTaskState = "COMPLETED"
	BatchTaskTerminated BatchTaskState = "TERMINATED"
)

// SelectRuleReq 对齐  SelectRuleReq。
// 真实策略数据后续来自 DB/Redis，这里先固定兼容字段，避免接口形态漂移。
type SelectRuleReq struct {
	MerchantID  int    `json:"merchantId"`
	RiskID      int    `json:"riskId,omitempty"`
	Callee      string `json:"callee"`
	ExtensionID int    `json:"extensionId,omitempty"`
	UserID      int    `json:"userId,omitempty"`
}

// SelectPhoneResp 对齐  SelectPhoneResp。
// CTI 选号成功后返回给 ESL 起呼使用，字段保持与  侧 JSON 名称一致。
type SelectPhoneResp struct {
	Phone              string `json:"phone"`
	GatewayID          int    `json:"gatewayId"`
	SkillGroupID       int    `json:"skillGroupId,omitempty"`
	ChannelID          int    `json:"channelId,omitempty"`
	GatewayName        string `json:"gatewayName,omitempty"`
	GatewayRegion      string `json:"gatewayRegion,omitempty"`
	Model              int    `json:"model,omitempty"`
	Extension          bool   `json:"extension,omitempty"`
	CallerPrefix       string `json:"callerPrefix,omitempty"`
	CalleePrefix       string `json:"calleePrefix,omitempty"`
	CallerRewriteRule  string `json:"callerRewriteRule,omitempty"`
	CalleeRewriteRule  string `json:"calleeRewriteRule,omitempty"`
	SupplementRing     bool   `json:"supplementRing,omitempty"`
	SupplementRingFile string `json:"supplementRingFile,omitempty"`
	Province           string `json:"province,omitempty"`
	City               string `json:"city,omitempty"`
	PoolID             int    `json:"poolId,omitempty"`
	CodecPrefs         string `json:"codecPrefs,omitempty"`
	BroadcastTime      int64  `json:"broadcastTime,omitempty"`
	BroadcastTimeFlag  bool   `json:"broadcastTimeFlag,omitempty"`
}

// CallControlReq 是 ESL 通话控制接口的兼容请求。
// 不同控制动作共用同一个 Go 结构，transport 层按 path 写入 Command 字段。
type CallControlReq struct {
	Command     string         `json:"command,omitempty"`
	CommandID   string         `json:"commandId,omitempty"`
	CallID      string         `json:"callId"`
	UUID        string         `json:"uuid,omitempty"`
	UUID1       string         `json:"uuid1,omitempty"`
	UUID2       string         `json:"uuid2,omitempty"`
	FSAddr      string         `json:"fsAddr,omitempty"`
	LegRole     LegRole        `json:"legRole,omitempty"`
	Digit       string         `json:"digit,omitempty"`
	Digits      string         `json:"digits,omitempty"`
	Destination string         `json:"destination,omitempty"`
	ReasonCode  string         `json:"reasonCode,omitempty"`
	CustomCause string         `json:"customCause,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// CallEavesdropReq 是 CTI 监听/强插接口的外部请求
type CallEavesdropReq struct {
	UserID       int    `json:"userId" binding:"required"`
	TargetCallID string `json:"targetCallId" binding:"required"`
	TargetUUID   string `json:"targetUuid,omitempty"`    // 可选，如果为空则由系统匹配活跃通道
	Mode         string `json:"mode" binding:"required"` // spy, whisper, barge
}

// CallHangupReq 是 CTI 强拆接口的外部请求
type CallHangupReq struct {
	CallID string `json:"callId" binding:"required"`
	UUID   string `json:"uuid,omitempty"` // 可选，为空则挂断整个呼叫的所有活跃通道
}
