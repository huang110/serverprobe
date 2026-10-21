package model

import "time"

// AlertRule 告警规则。
// AgentID 为空表示全局规则。
type AlertRule struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	AgentID    string    `gorm:"size:64;index:idx_agent_id" json:"agent_id"`
	Metric     string    `gorm:"size:50;not null" json:"metric"`
	Condition  string    `gorm:"size:20;not null" json:"condition"`
	Threshold  float64   `gorm:"not null" json:"threshold"`
	Duration   int32     `gorm:"not null;default:60" json:"duration"`
	Severity   string    `gorm:"size:20;not null;default:'warning'" json:"severity"`
	Enabled    bool      `gorm:"not null;default:true;index:idx_enabled" json:"enabled"`
	Expression string    `gorm:"type:text" json:"expression"`
	NotifyURL  string    `gorm:"size:500" json:"notify_url"`
	WebhookURL string    `gorm:"size:500" json:"webhook_url"`
	Message    string    `gorm:"size:500" json:"message"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定告警规则表名
func (AlertRule) TableName() string {
	return "alert_rules"
}

// AlertRecord 告警记录。
type AlertRecord struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RuleID      uint64    `gorm:"not null;index:idx_rule_id" json:"rule_id"`
	AgentID     string    `gorm:"size:64;not null;index:idx_alert_records_agent_id" json:"agent_id"`
	Metric      string    `gorm:"size:50;not null" json:"metric"`
	Threshold   float64   `gorm:"not null" json:"threshold"`
	ActualValue float64   `gorm:"not null" json:"actual_value"`
	Severity    string    `gorm:"size:20;not null;index:idx_severity" json:"severity"`
	Message     string    `gorm:"type:text" json:"message"`
	Status      string    `gorm:"size:20;not null;index:idx_alert_records_status;default:'firing'" json:"status"`
	FiredAt     int64     `gorm:"not null;index:idx_fired_at" json:"fired_at"`
	ResolvedAt  int64     `gorm:"index:idx_resolved_at" json:"resolved_at"`
	NotifiedAt  int64     `json:"notified_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定告警记录表名
func (AlertRecord) TableName() string {
	return "alert_records"
}
