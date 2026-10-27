package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

// MetricsHandler 指标查询 HTTP 接口
type MetricsHandler struct {
	db    *gorm.DB
	redis *cache.RedisClient
}

// NewMetricsHandler 创建指标查询接口
func NewMetricsHandler(db *gorm.DB, redisClient *cache.RedisClient) *MetricsHandler {
	return &MetricsHandler{db: db, redis: redisClient}
}

// MetricPoint 历史指标点
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// LatencyPoint TCP Ping 延迟历史点
type LatencyPoint struct {
	Timestamp  int64   `json:"timestamp"`
	LatencyMs  float64 `json:"latency_ms"`
	JitterMs   float64 `json:"jitter_ms"`
	PacketLoss float64 `json:"packet_loss"`
}

// LatencySeries 单个 TCP Ping 目标的历史序列
type LatencySeries struct {
	TargetName string         `json:"target_name"`
	Target     string         `json:"target"`
	Points     []LatencyPoint `json:"points"`
}

// GetRealtimeMetrics GET /api/agents/:id/metrics/realtime
func (h *MetricsHandler) GetRealtimeMetrics(c *gin.Context) {
	agentID := c.Param("id")
	if h.redis == nil {
		respondError(c, http.StatusNotFound, "no realtime metrics available")
		return
	}

	metrics, err := h.redis.GetRealtimeMetrics(agentID)
	if err != nil || len(metrics) == 0 {
		respondError(c, http.StatusNotFound, "no realtime metrics available")
		return
	}

	diskData := make(map[string]map[string]interface{})
	diskMetrics, err := h.redis.GetDiskMetrics(agentID)
	if err == nil {
		for mountPoint, payload := range diskMetrics {
			var data map[string]interface{}
			if json.Unmarshal([]byte(payload), &data) == nil {
				diskData[mountPoint] = data
			}
		}
	}

	networkData := make(map[string]map[string]interface{})
	networkMetrics, err := h.redis.GetNetworkMetrics(agentID)
	if err == nil {
		for interfaceName, payload := range networkMetrics {
			var data map[string]interface{}
			if json.Unmarshal([]byte(payload), &data) == nil {
				networkData[interfaceName] = data
			}
		}
	}

	latencyItems := make([]model.LatencyItem, 0)
	if items, latencyErr := h.redis.GetLatencyItems(agentID); latencyErr == nil {
		latencyItems = items
	}

	respondOK(c, gin.H{
		"cpu_usage":         parseFloat(metrics["cpu_usage"]),
		"cpu_steal":         parseFloat(metrics["cpu_steal"]),
		"cpu_iowait":        parseFloat(metrics["cpu_iowait"]),
		"memory_used":       parseInt64Default(metrics["memory_used"], 0),
		"memory_percent":    parseFloat(metrics["memory_percent"]),
		"swap_total":        parseInt64Default(metrics["swap_total"], 0),
		"swap_used":         parseInt64Default(metrics["swap_used"], 0),
		"swap_percent":      parseFloat(metrics["swap_percent"]),
		"load1":             parseFloat(metrics["load1"]),
		"load5":             parseFloat(metrics["load5"]),
		"load15":            parseFloat(metrics["load15"]),
		"net_rx_bytes":      parseInt64Default(metrics["net_rx_bytes"], 0),
		"net_tx_bytes":      parseInt64Default(metrics["net_tx_bytes"], 0),
		"net_rx_total":      parseInt64Default(metrics["net_rx_total"], 0),
		"net_tx_total":      parseInt64Default(metrics["net_tx_total"], 0),
		"uptime":            parseInt64Default(metrics["uptime"], 0),
		"process_count":     parseInt64Default(metrics["process_count"], 0),
		"tcp_established":   parseInt64Default(metrics["tcp_established"], 0),
		"tcp_time_wait":     parseInt64Default(metrics["tcp_time_wait"], 0),
		"status":            metrics["status"],
		"last_heartbeat_at": parseInt64Default(metrics["last_heartbeat_at"], 0),
		"disk":              diskData,
		"network":           networkData,
		"latency":           latencyItems,
	})
}

// GetLatencyHistory GET /api/agents/:id/metrics/latency
func (h *MetricsHandler) GetLatencyHistory(c *gin.Context) {
	agentID := c.Param("id")

	now := time.Now().Unix()
	start := parseInt64Default(c.Query("start"), now-3600)
	end := parseInt64Default(c.Query("end"), now)
	if end <= start {
		respondError(c, http.StatusBadRequest, "invalid time range")
		return
	}

	type targetRow struct {
		TargetName string
		Target     string
	}
	targets := make([]targetRow, 0)
	if err := h.db.Model(&model.MetricsLatency{}).
		Select("target_name, MAX(target) AS target").
		Where("agent_id = ? AND timestamp >= ? AND timestamp <= ?", agentID, start, end).
		Group("target_name").
		Order("target_name ASC").
		Scan(&targets).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query latency metrics")
		return
	}

	const maxPointsPerSeries = 2000
	series := make([]LatencySeries, 0, len(targets))
	for _, target := range targets {
		points := make([]LatencyPoint, 0)
		if err := h.db.Model(&model.MetricsLatency{}).
			Select("timestamp, latency_ms, jitter_ms, packet_loss").
			Where("agent_id = ? AND target_name = ? AND timestamp >= ? AND timestamp <= ?",
				agentID, target.TargetName, start, end).
			Order("timestamp ASC").
			Limit(maxPointsPerSeries).
			Scan(&points).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "failed to query latency metrics")
			return
		}
		series = append(series, LatencySeries{
			TargetName: target.TargetName,
			Target:     target.Target,
			Points:     points,
		})
	}

	respondOK(c, gin.H{
		"series": series,
	})
}

// GetHistoryMetrics GET /api/agents/:id/metrics/history
func (h *MetricsHandler) GetHistoryMetrics(c *gin.Context) {
	agentID := c.Param("id")
	metric := strings.ToLower(c.DefaultQuery("metric", "cpu"))
	if !isValidMetric(metric) {
		respondError(c, http.StatusBadRequest, "invalid metric")
		return
	}

	now := time.Now().Unix()
	start := parseInt64Default(c.Query("start"), now-3600)
	end := parseInt64Default(c.Query("end"), now)
	if end <= start {
		respondError(c, http.StatusBadRequest, "invalid time range")
		return
	}

	granularity := strings.ToLower(strings.TrimSpace(c.Query("granularity")))
	if granularity == "" {
		if metric == "disk" {
			// 磁盘没有预聚合表，默认使用 1m 原始数据
			granularity = "1m"
		} else {
			granularity = autoSelectGranularity(start, end)
		}
	}
	if !isValidGranularity(granularity) {
		respondError(c, http.StatusBadRequest, "invalid granularity")
		return
	}
	if metric == "disk" && granularity != "1m" {
		respondError(c, http.StatusBadRequest, "disk metric only supports 1m granularity")
		return
	}

	var points []MetricPoint
	var err error
	if granularity == "1m" {
		points, err = h.queryRawMetrics(agentID, metric, start, end)
	} else {
		points, err = h.queryAggregatedMetrics(agentID, metric, start, end, granularity)
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query metrics")
		return
	}
	if points == nil {
		points = make([]MetricPoint, 0)
	}

	respondOK(c, gin.H{
		"points":      points,
		"granularity": granularity,
	})
}

// queryRawMetrics 查询原始指标；原始指标按月份分布在多个分区表
func (h *MetricsHandler) queryRawMetrics(agentID, metric string, start, end int64) ([]MetricPoint, error) {
	if metric == "disk" {
		return h.queryTableSeries("metrics_disk", "usage_percent", agentID, start, end)
	}
	if metric == "network" {
		return h.queryTableSeries("metrics_network", "rx_bytes", agentID, start, end)
	}

	column, ok := rawMetricColumn(metric)
	if !ok {
		return nil, nil
	}

	points := make([]MetricPoint, 0)
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)

	for cursor := beginningOfMonth(startTime); !cursor.After(endTime); cursor = cursor.AddDate(0, 1, 0) {
		tableName := model.GetMetricsRawTableName(cursor)
		if !h.db.Migrator().HasTable(tableName) {
			continue
		}

		var rows []MetricPoint
		if err := h.db.Table(tableName).
			Select("timestamp, "+column+" AS value").
			Where("agent_id = ? AND timestamp >= ? AND timestamp <= ?", agentID, start, end).
			Order("timestamp ASC").
			Limit(10000).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		points = append(points, rows...)
	}

	return points, nil
}

// queryAggregatedMetrics 查询预聚合指标
func (h *MetricsHandler) queryAggregatedMetrics(agentID, metric string, start, end int64, granularity string) ([]MetricPoint, error) {
	column, ok := aggregatedMetricColumn(metric)
	if !ok {
		return nil, nil
	}

	var points []MetricPoint
	err := h.db.Table((model.MetricsAggregated{}).TableName()).
		Select("timestamp, "+column+" AS value").
		Where("agent_id = ? AND granularity = ? AND timestamp >= ? AND timestamp <= ?",
			agentID, granularity, start, end).
		Order("timestamp ASC").
		Limit(10000).
		Scan(&points).Error
	return points, err
}

// queryTableSeries 查询非分区明细表
func (h *MetricsHandler) queryTableSeries(table, column, agentID string, start, end int64) ([]MetricPoint, error) {
	var points []MetricPoint
	err := h.db.Table(table).
		Select("timestamp, "+column+" AS value").
		Where("agent_id = ? AND timestamp >= ? AND timestamp <= ?", agentID, start, end).
		Order("timestamp ASC").
		Limit(10000).
		Scan(&points).Error
	return points, err
}

func beginningOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func autoSelectGranularity(start, end int64) string {
	duration := end - start
	switch {
	case duration <= 3600:
		return "1m"
	case duration <= 86400:
		return "5m"
	case duration <= 604800:
		return "1h"
	default:
		return "1d"
	}
}

func isValidGranularity(granularity string) bool {
	switch granularity {
	case "1m", "5m", "1h", "1d":
		return true
	default:
		return false
	}
}

func isValidMetric(metric string) bool {
	switch metric {
	case "cpu", "memory", "load", "disk", "network":
		return true
	default:
		return false
	}
}

func rawMetricColumn(metric string) (string, bool) {
	switch metric {
	case "cpu":
		return "cpu_usage", true
	case "memory":
		return "memory_percent", true
	case "load":
		return "load1", true
	default:
		return "", false
	}
}

func aggregatedMetricColumn(metric string) (string, bool) {
	switch metric {
	case "cpu":
		return "cpu_avg", true
	case "memory":
		return "memory_avg", true
	case "load":
		return "load1_avg", true
	case "network":
		return "net_rx_bytes_sum", true
	default:
		return "", false
	}
}
