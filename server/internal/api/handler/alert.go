package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/model"
)

// AlertHandler 告警管理 HTTP 接口
type AlertHandler struct {
	db *gorm.DB
}

// NewAlertHandler 创建告警管理接口
func NewAlertHandler(db *gorm.DB) *AlertHandler {
	return &AlertHandler{db: db}
}

// CreateRule POST /api/alert/rules
func (h *AlertHandler) CreateRule(c *gin.Context) {
	var rule model.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	rule.Name = strings.TrimSpace(rule.Name)
	rule.Metric = strings.TrimSpace(rule.Metric)
	rule.Condition = strings.TrimSpace(rule.Condition)
	rule.Severity = strings.TrimSpace(rule.Severity)

	if rule.Name == "" || rule.Metric == "" {
		respondError(c, http.StatusBadRequest, "name and metric are required")
		return
	}
	if !isValidAlertMetric(rule.Metric) {
		respondError(c, http.StatusBadRequest, "invalid metric")
		return
	}
	if !isValidAlertCondition(rule.Condition) {
		respondError(c, http.StatusBadRequest, "invalid condition")
		return
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if !isValidSeverity(rule.Severity) {
		respondError(c, http.StatusBadRequest, "invalid severity")
		return
	}
	if rule.Duration <= 0 {
		rule.Duration = 60
	}

	// 防止客户端指定主键和审计字段
	rule.ID = 0
	rule.Enabled = true
	rule.CreatedAt = time.Time{}
	rule.UpdatedAt = time.Time{}

	if err := h.db.Create(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create rule")
		return
	}

	respondOK(c, rule)
}

// ListRules GET /api/alert/rules
func (h *AlertHandler) ListRules(c *gin.Context) {
	agentID := strings.TrimSpace(c.Query("agent_id"))
	enabled := c.DefaultQuery("enabled", "all")
	if enabled != "all" && enabled != "true" && enabled != "false" {
		respondError(c, http.StatusBadRequest, "invalid enabled filter")
		return
	}

	query := h.db.Model(&model.AlertRule{})
	if agentID != "" {
		query = query.Where("agent_id = ? OR agent_id = ''", agentID)
	}
	if enabled != "all" {
		query = query.Where("enabled = ?", enabled == "true")
	}

	rules := make([]model.AlertRule, 0)
	if err := query.Order("created_at DESC").Find(&rules).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query rules")
		return
	}

	respondOK(c, rules)
}

// ListRecords GET /api/alert/records
func (h *AlertHandler) ListRecords(c *gin.Context) {
	agentID := strings.TrimSpace(c.Query("agent_id"))
	severity := strings.TrimSpace(c.Query("severity"))
	status := strings.TrimSpace(c.Query("status"))

	if severity != "" && !isValidSeverity(severity) {
		respondError(c, http.StatusBadRequest, "invalid severity")
		return
	}
	if status != "" && status != "firing" && status != "resolved" {
		respondError(c, http.StatusBadRequest, "invalid status")
		return
	}

	now := time.Now().Unix()
	start := parseInt64Default(c.Query("start"), now-24*3600)
	end := parseInt64Default(c.Query("end"), now)
	if end <= start {
		respondError(c, http.StatusBadRequest, "invalid time range")
		return
	}

	query := h.db.Model(&model.AlertRecord{}).
		Where("fired_at >= ? AND fired_at <= ?", start, end)
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	records := make([]model.AlertRecord, 0)
	if err := query.Order("fired_at DESC").Limit(100).Find(&records).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query records")
		return
	}

	respondOK(c, records)
}

func isValidAlertMetric(metric string) bool {
	switch metric {
	case "cpu_usage", "memory_percent", "disk_usage", "load1":
		return true
	default:
		return false
	}
}

func isValidAlertCondition(condition string) bool {
	switch condition {
	case "gt", "lt", "gte", "lte", "eq":
		return true
	default:
		return false
	}
}

func isValidSeverity(severity string) bool {
	switch severity {
	case "info", "warning", "critical":
		return true
	default:
		return false
	}
}
