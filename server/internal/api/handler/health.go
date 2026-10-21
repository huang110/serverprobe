package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
)

// HealthHandler 健康检查接口
type HealthHandler struct {
	db    *gorm.DB
	redis *cache.RedisClient
}

// NewHealthHandler 创建健康检查接口
func NewHealthHandler(db *gorm.DB, redisClient *cache.RedisClient) *HealthHandler {
	return &HealthHandler{db: db, redis: redisClient}
}

// Health GET /api/health
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// Ready GET /api/health/ready
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := make(map[string]string)

	if h.db == nil {
		checks["mysql"] = "fail: database is not initialized"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	if err := h.db.Exec("SELECT 1").Error; err != nil {
		checks["mysql"] = "fail: " + err.Error()
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	checks["mysql"] = "ok"

	if h.redis == nil || h.redis.Client() == nil {
		checks["redis"] = "fail: redis is not initialized"
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	if err := h.redis.Client().Ping(c.Request.Context()).Err(); err != nil {
		checks["redis"] = "fail: " + err.Error()
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
		return
	}
	checks["redis"] = "ok"

	c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
}

// Metrics GET /api/health/metrics
// 简化版 Prometheus 文本指标，避免引入额外依赖。
func (h *HealthHandler) Metrics(c *gin.Context) {
	online := 0
	if h.redis != nil {
		if agents, err := h.redis.GetOnlineAgents(300); err == nil {
			online = len(agents)
		}
	}

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.Status(http.StatusOK)
	_, _ = fmt.Fprintf(c.Writer, "# HELP serverprobe_up ServerProbe server is up\n")
	_, _ = fmt.Fprintf(c.Writer, "# TYPE serverprobe_up gauge\n")
	_, _ = fmt.Fprintf(c.Writer, "serverprobe_up 1\n")
	_, _ = fmt.Fprintf(c.Writer, "# HELP serverprobe_agents_online Number of online agents\n")
	_, _ = fmt.Fprintf(c.Writer, "# TYPE serverprobe_agents_online gauge\n")
	_, _ = fmt.Fprintf(c.Writer, "serverprobe_agents_online %d\n", online)
}
