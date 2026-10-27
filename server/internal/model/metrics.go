package model

import "time"

// MetricsRaw 原始指标数据。
// 实际存储时按月份映射到 metrics_raw_YYYYMM 分区表。
// 分区表的索引由 CreateMetricsRawPartitions 按分区创建，避免 SQLite 等
// 数据库要求索引名全局唯一而导致的跨分区冲突。
type MetricsRaw struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID   string `gorm:"size:64;not null" json:"agent_id"`
	Timestamp int64  `gorm:"not null" json:"timestamp"`

	// CPU
	CPUUsage  float64 `json:"cpu_usage"`
	CPUUser   float64 `json:"cpu_user"`
	CPUSystem float64 `json:"cpu_system"`

	// Memory
	MemoryUsed    int64   `json:"memory_used"`
	MemoryPercent float64 `json:"memory_percent"`
	SwapUsed      int64   `json:"swap_used"`
	SwapPercent   float64 `json:"swap_percent"`

	// Load
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`

	// Network（所有网卡总和）
	NetRxBytes int64 `json:"net_rx_bytes"`
	NetTxBytes int64 `json:"net_tx_bytes"`

	// Connections
	TCPEstab    int32 `json:"tcp_estab"`
	TCPTimeWait int32 `json:"tcp_time_wait"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 返回默认表名；分区表通过 db.Table() 动态指定。
func (MetricsRaw) TableName() string {
	return "metrics_raw"
}

// MetricsAggregated 聚合指标数据。
type MetricsAggregated struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID     string `gorm:"size:64;not null;index:idx_agent_granularity_time,priority:1" json:"agent_id"`
	Granularity string `gorm:"size:10;not null;index:idx_agent_granularity_time,priority:2" json:"granularity"`
	Timestamp   int64  `gorm:"not null;index:idx_agent_granularity_time,priority:3" json:"timestamp"`

	CPUAvg        float64 `json:"cpu_avg"`
	CPUMax        float64 `json:"cpu_max"`
	MemoryAvg     int64   `json:"memory_avg"`
	MemoryMax     int64   `json:"memory_max"`
	Load1Avg      float64 `json:"load1_avg"`
	Load1Max      float64 `json:"load1_max"`
	NetRxBytesSum int64   `json:"net_rx_bytes_sum"`
	NetTxBytesSum int64   `json:"net_tx_bytes_sum"`

	TCPEstablishedAvg float64 `json:"tcp_established_avg"`

	SampleCount int32     `json:"sample_count"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定聚合指标表名
func (MetricsAggregated) TableName() string {
	return "metrics_aggregated"
}

// MetricsDisk 磁盘指标。
type MetricsDisk struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID    string `gorm:"size:64;not null;index:idx_agent_mount_time,priority:1" json:"agent_id"`
	MountPoint string `gorm:"size:255;not null;index:idx_agent_mount_time,priority:2" json:"mount_point"`
	Timestamp  int64  `gorm:"not null;index:idx_agent_mount_time,priority:3" json:"timestamp"`

	Device       string  `gorm:"size:100" json:"device"`
	FsType       string  `gorm:"size:50" json:"fs_type"`
	Total        int64   `json:"total"`
	Used         int64   `json:"used"`
	UsagePercent float64 `json:"usage_percent"`
	InodePercent float64 `json:"inode_percent"`
	ReadBytes    float64 `json:"read_bytes"`
	WriteBytes   float64 `json:"write_bytes"`
	IOUtil       float64 `json:"io_util"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定磁盘指标表名
func (MetricsDisk) TableName() string {
	return "metrics_disk"
}

// MetricsNetwork 网络指标。
type MetricsNetwork struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID   string `gorm:"size:64;not null;index:idx_agent_iface_time,priority:1" json:"agent_id"`
	Interface string `gorm:"size:100;not null;index:idx_agent_iface_time,priority:2" json:"interface"`
	Timestamp int64  `gorm:"not null;index:idx_agent_iface_time,priority:3" json:"timestamp"`

	IPAddress string `gorm:"size:45" json:"ip_address"`
	RxBytes   int64  `json:"rx_bytes"`
	TxBytes   int64  `json:"tx_bytes"`
	RxPackets int64  `json:"rx_packets"`
	TxPackets int64  `json:"tx_packets"`
	RxErrors  int64  `json:"rx_errors"`
	TxErrors  int64  `json:"tx_errors"`

	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定网络指标表名
func (MetricsNetwork) TableName() string {
	return "metrics_network"
}

// MetricsLatency TCP Ping 延迟历史明细
type MetricsLatency struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID    string    `gorm:"size:64;not null;index:idx_agent_target_time,priority:1" json:"agent_id"`
	TargetName string    `gorm:"size:100;not null;index:idx_agent_target_time,priority:2" json:"target_name"`
	Target     string    `gorm:"size:255;not null" json:"target"`
	Timestamp  int64     `gorm:"not null;index:idx_agent_target_time,priority:3" json:"timestamp"`
	LatencyMs  float64   `json:"latency_ms"`
	JitterMs   float64   `json:"jitter_ms"`
	PacketLoss float64   `json:"packet_loss"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定延迟指标表名
func (MetricsLatency) TableName() string {
	return "metrics_latency"
}

// LatencyItem 单个 TCP Ping 目标的实时/历史数据，JSON 字段与前后端契约一致
type LatencyItem struct {
	TargetName string  `json:"target_name"`
	Target     string  `json:"target"`
	LatencyMs  float64 `json:"latency_ms"`
	JitterMs   float64 `json:"jitter_ms"`
	PacketLoss float64 `json:"packet_loss"`
}
