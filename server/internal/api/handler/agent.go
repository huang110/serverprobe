package handler

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/middleware"
	"github.com/huang110/serverprobe/server/internal/model"
)

// AgentHandler Agent 管理 HTTP 接口
type AgentHandler struct {
	db    *gorm.DB
	redis *cache.RedisClient
}

// NewAgentHandler 创建 Agent 管理接口
func NewAgentHandler(db *gorm.DB, redisClient *cache.RedisClient) *AgentHandler {
	return &AgentHandler{db: db, redis: redisClient}
}

// ListAgents GET /api/agents
func (h *AgentHandler) ListAgents(c *gin.Context) {
	status := c.DefaultQuery("status", "all")
	if status != "all" && status != "online" && status != "offline" {
		respondError(c, http.StatusBadRequest, "invalid status")
		return
	}

	keyword := strings.TrimSpace(c.Query("keyword"))
	page, pageSize := paginationParams(c)

	query := h.db.Model(&model.Agent{})
	if status != "all" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(hostname LIKE ? OR ip LIKE ?)", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to count agents")
		return
	}

	agents := make([]model.Agent, 0)
	if err := query.
		Order("last_heartbeat_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&agents).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query agents")
		return
	}

	if !middleware.IsAuthenticated(c) {
		for index := range agents {
			maskAgentIP(&agents[index])
		}
	}

	respondOK(c, gin.H{
		"agents":    agents,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetAgent GET /api/agents/:id
func (h *AgentHandler) GetAgent(c *gin.Context) {
	agentID := c.Param("id")

	var agent model.Agent
	if err := h.db.First(&agent, "id = ?", agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "agent not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}

	labels, err := h.loadLabels(agentID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to load labels")
		return
	}

	realtime := make(map[string]string)
	if h.redis != nil {
		realtime, err = h.redis.GetRealtimeMetrics(agentID)
		if err != nil {
			log.Printf("warning: failed to get realtime metrics for %s: %v", agentID, err)
			realtime = make(map[string]string)
		}
	}

	if !middleware.IsAuthenticated(c) {
		maskAgentIP(&agent)
	}

	respondOK(c, gin.H{
		"agent":    agent,
		"labels":   labels,
		"realtime": realtime,
	})
}

// UpdateAgent PUT /api/agents/:id
func (h *AgentHandler) UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")

	var request struct {
		Labels map[string]string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	var agent model.Agent
	if err := h.db.First(&agent, "id = ?", agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "agent not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		return replaceAgentLabels(tx, agentID, request.Labels)
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update labels")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "agent updated",
	})
}

// DeleteAgent DELETE /api/agents/:id
func (h *AgentHandler) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&model.AgentLabel{}).Error; err != nil {
			return err
		}

		result := tx.Where("id = ?", agentID).Delete(&model.Agent{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "agent not found")
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to delete agent")
		return
	}

	if h.redis != nil {
		if err := h.redis.DeleteAgentCache(agentID); err != nil {
			log.Printf("warning: failed to clear cache for agent %s: %v", agentID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "agent deleted",
	})
}

// loadLabels 查询并转换为 map
func (h *AgentHandler) loadLabels(agentID string) (map[string]string, error) {
	var labels []model.AgentLabel
	if err := h.db.Where("agent_id = ?", agentID).Find(&labels).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string, len(labels))
	for _, label := range labels {
		result[label.Key] = label.Value
	}
	return result, nil
}

// replaceAgentLabels 在事务中替换 Agent 的全部标签
func replaceAgentLabels(tx *gorm.DB, agentID string, labels map[string]string) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&model.AgentLabel{}).Error; err != nil {
		return err
	}

	normalized := make(map[string]string, len(labels))
	for key, value := range labels {
		key = strings.TrimSpace(key)
		if key != "" {
			normalized[key] = value
		}
	}
	if len(normalized) == 0 {
		return nil
	}

	records := make([]model.AgentLabel, 0, len(normalized))
	for key, value := range normalized {
		records = append(records, model.AgentLabel{
			AgentID: agentID,
			Key:     key,
			Value:   value,
		})
	}
	return tx.Create(&records).Error
}

// maskAgentIP 对游客隐藏 Agent 的精确 IP 和内网 IP
func maskAgentIP(agent *model.Agent) {
	if agent == nil {
		return
	}
	agent.IP = maskIP(agent.IP)
	agent.InternalIP = ""
}

// maskIP 对 IPv4/IPv6 地址进行脱敏。
// IPv4 保留前两段；IPv6 保留前两段 hextet。
func maskIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.*.*", ipv4[0], ipv4[1])
	}

	ipv6 := ip.To16()
	if ipv6 == nil {
		return ""
	}
	first := uint16(ipv6[0])<<8 | uint16(ipv6[1])
	second := uint16(ipv6[2])<<8 | uint16(ipv6[3])
	return fmt.Sprintf("%x:%x:*:*:*:*:*:*", first, second)
}
