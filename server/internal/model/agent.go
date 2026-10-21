package model

import "time"

// Agent Agent 注册信息。
// Token 不参与 JSON 序列化，避免通过 HTTP API 泄露。
type Agent struct {
	ID              string    `gorm:"primaryKey;size:64" json:"id"`
	Hostname        string    `gorm:"size:255;not null;index:idx_hostname" json:"hostname"`
	IP              string    `gorm:"size:45;index:idx_ip" json:"ip"`
	InternalIP      string    `gorm:"size:45" json:"internal_ip"`
	OS              string    `gorm:"size:50" json:"os"`
	OSVersion       string    `gorm:"size:100" json:"os_version"`
	KernelVersion   string    `gorm:"size:100" json:"kernel_version"`
	CPUCores        int32     `gorm:"not null" json:"cpu_cores"`
	MemoryTotal     int64     `gorm:"not null" json:"memory_total"`
	DiskTotal       int64     `gorm:"not null" json:"disk_total"`
	Arch            string    `gorm:"size:20" json:"arch"`
	AgentVersion    string    `gorm:"size:50" json:"agent_version"`
	Token           string    `gorm:"size:128;not null" json:"-"`
	TokenExpiresAt  int64     `gorm:"not null;index:idx_token_expires_at" json:"token_expires_at"`
	LastHeartbeatAt int64     `gorm:"not null;index:idx_last_heartbeat_at" json:"last_heartbeat_at"`
	Status          string    `gorm:"size:20;not null;index:idx_status;default:'online'" json:"status"`
	LiteMode        bool      `gorm:"not null;default:false" json:"lite_mode"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定 Agent 表名
func (Agent) TableName() string {
	return "agents"
}

// AgentLabel Agent 自定义标签。
// agent_id + key 组成联合唯一索引。
type AgentLabel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AgentID   string    `gorm:"size:64;not null;uniqueIndex:idx_agent_key,priority:1" json:"agent_id"`
	Key       string    `gorm:"size:100;not null;uniqueIndex:idx_agent_key,priority:2" json:"key"`
	Value     string    `gorm:"size:255;not null" json:"value"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定 AgentLabel 表名
func (AgentLabel) TableName() string {
	return "agent_labels"
}
