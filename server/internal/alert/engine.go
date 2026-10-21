package alert

import (
	"context"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
)

const (
	// defaultCheckInterval 默认告警检查间隔
	defaultCheckInterval = 30 * time.Second

	// onlineThreshold 在线 Agent 判定阈值（秒）
	onlineThreshold = 300
)

// Engine 告警引擎
type Engine struct {
	db        *gorm.DB
	redis     *cache.RedisClient
	config    *config.Config
	evaluator *Evaluator
	notifier  *Notifier

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	started  bool
}

// NewEngine 创建告警引擎
func NewEngine(db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *Engine {
	return &Engine{
		db:        db,
		redis:     redisClient,
		config:    cfg,
		evaluator: NewEvaluator(redisClient),
		notifier:  NewNotifier(cfg),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start 启动告警引擎，阻塞运行直到 Stop
func (e *Engine) Start() {
	e.mu.Lock()
	if e.started {
		e.mu.Unlock()
		return
	}
	e.started = true
	e.mu.Unlock()

	defer close(e.doneCh)

	interval := defaultCheckInterval
	if e.config != nil {
		if seconds := e.config.GetAlertCheckInterval(); seconds > 0 {
			interval = time.Duration(seconds) * time.Second
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动后立即检查一次，之后按周期执行
	e.checkRules(context.Background())

	for {
		select {
		case <-ticker.C:
			e.checkRules(context.Background())
		case <-e.stopCh:
			return
		}
	}
}

// Stop 停止告警引擎
func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopCh)
	})

	e.mu.Lock()
	started := e.started
	e.mu.Unlock()
	if started {
		<-e.doneCh
	}
}

// checkRules 扫描所有启用的规则
func (e *Engine) checkRules(ctx context.Context) {
	if e.db == nil {
		return
	}

	var rules []model.AlertRule
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&rules).Error; err != nil {
		log.Printf("failed to load alert rules: %v", err)
		return
	}

	for i := range rules {
		rule := rules[i]
		if rule.AgentID != "" {
			e.evaluateRuleForAgent(ctx, &rule, rule.AgentID)
			continue
		}

		if e.redis == nil {
			continue
		}
		agents, err := e.redis.GetOnlineAgents(onlineThreshold)
		if err != nil {
			log.Printf("failed to get online agents: %v", err)
			continue
		}
		for _, agentID := range agents {
			e.evaluateRuleForAgent(ctx, &rule, agentID)
		}
	}
}

// evaluateRuleForAgent 评估单个 Agent 的规则
func (e *Engine) evaluateRuleForAgent(ctx context.Context, rule *model.AlertRule, agentID string) {
	result, value, err := e.evaluator.Evaluate(ctx, rule, agentID)
	if err != nil {
		return
	}

	switch result {
	case ResultFiring:
		e.handleFiring(ctx, rule, agentID, value)
	case ResultResolved:
		e.handleResolved(ctx, rule, agentID, value)
	}
}

// handleFiring 处理告警触发
func (e *Engine) handleFiring(ctx context.Context, rule *model.AlertRule, agentID string, value float64) {
	hostname := e.agentHostname(ctx, agentID)
	now := time.Now().Unix()
	message := buildAlertMessage(rule, value)

	record := model.AlertRecord{
		RuleID:      rule.ID,
		AgentID:     agentID,
		Metric:      rule.Metric,
		Threshold:   rule.Threshold,
		ActualValue: value,
		Severity:    rule.Severity,
		Message:     message,
		Status:      "firing",
		FiredAt:     now,
	}
	if err := e.db.WithContext(ctx).Create(&record).Error; err != nil {
		log.Printf("failed to create alert record: %v", err)
		return
	}

	if err := e.notifier.Notify(ctx, rule, agentID, hostname, value, "firing"); err != nil {
		log.Printf("failed to send firing notification: %v", err)
	} else {
		if err := e.db.WithContext(ctx).Model(&model.AlertRecord{}).
			Where("id = ?", record.ID).
			Update("notified_at", now).Error; err != nil {
			log.Printf("failed to update notified_at: %v", err)
		}
	}

	e.publishEvent(ctx, rule, agentID, value, "firing", now)
	log.Printf("alert fired: rule=%s agent=%s metric=%s value=%.2f threshold=%.2f",
		rule.Name, agentID, rule.Metric, value, rule.Threshold)
}

// handleResolved 处理告警恢复
func (e *Engine) handleResolved(ctx context.Context, rule *model.AlertRule, agentID string, value float64) {
	hostname := e.agentHostname(ctx, agentID)
	now := time.Now().Unix()

	var record model.AlertRecord
	err := e.db.WithContext(ctx).
		Where("rule_id = ? AND agent_id = ? AND status = ?", rule.ID, agentID, "firing").
		Order("fired_at DESC").
		First(&record).Error
	if err != nil {
		log.Printf("failed to find firing alert record: %v", err)
		return
	}

	if err := e.db.WithContext(ctx).Model(&model.AlertRecord{}).
		Where("id = ?", record.ID).
		Updates(map[string]interface{}{
			"status":      "resolved",
			"resolved_at": now,
		}).Error; err != nil {
		log.Printf("failed to update alert record: %v", err)
		return
	}

	if err := e.notifier.Notify(ctx, rule, agentID, hostname, value, "resolved"); err != nil {
		log.Printf("failed to send resolved notification: %v", err)
	}

	e.publishEvent(ctx, rule, agentID, value, "resolved", now)
	log.Printf("alert resolved: rule=%s agent=%s metric=%s", rule.Name, agentID, rule.Metric)
}

// publishEvent 发布告警事件到 Redis
func (e *Engine) publishEvent(ctx context.Context, rule *model.AlertRule, agentID string, value float64, status string, firedAt int64) {
	if e.redis == nil {
		return
	}
	event := cache.AgentEvent{
		Type:     cache.EventTypeAlert,
		AgentID:  agentID,
		RuleID:   rule.ID,
		Severity: rule.Severity,
		Message:  buildAlertMessage(rule, value),
		Status:   status,
		FiredAt:  firedAt,
	}
	if err := e.redis.PublishEvent(ctx, event); err != nil {
		log.Printf("failed to publish alert event: %v", err)
	}
}

// agentHostname 查询 Agent 主机名
func (e *Engine) agentHostname(ctx context.Context, agentID string) string {
	var agent model.Agent
	if err := e.db.WithContext(ctx).Select("hostname").First(&agent, "id = ?", agentID).Error; err != nil {
		return agentID
	}
	if agent.Hostname == "" {
		return agentID
	}
	return agent.Hostname
}
