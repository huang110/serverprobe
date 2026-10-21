package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
)

// webhookTimeout Webhook 请求超时
const webhookTimeout = 5 * time.Second

// Notifier 告警通知发送器
type Notifier struct {
	config *config.Config
	client *http.Client
}

// NewNotifier 创建通知发送器
func NewNotifier(cfg *config.Config) *Notifier {
	return &Notifier{
		config: cfg,
		client: &http.Client{Timeout: webhookTimeout},
	}
}

// Notify 发送告警通知，未配置 Webhook 时直接返回
func (n *Notifier) Notify(ctx context.Context, rule *model.AlertRule, agentID, hostname string, value float64, status string) error {
	if rule == nil {
		return nil
	}

	webhookURL := rule.WebhookURL
	if webhookURL == "" {
		webhookURL = rule.NotifyURL
	}
	if webhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"rule_id":        rule.ID,
		"rule_name":      rule.Name,
		"agent_id":       agentID,
		"agent_hostname": hostname,
		"metric":         rule.Metric,
		"condition":      rule.Condition,
		"threshold":      rule.Threshold,
		"current_value":  value,
		"severity":       rule.Severity,
		"status":         status,
		"fired_at":       time.Now().Unix(),
		"message":        buildAlertMessage(rule, value),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := n.client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook returned status %d", response.StatusCode)
	}
	return nil
}

// buildAlertMessage 构造告警描述
func buildAlertMessage(rule *model.AlertRule, value float64) string {
	if rule.Message != "" {
		return rule.Message
	}
	return fmt.Sprintf("%s %s %.2f (current %.2f)", rule.Metric, rule.Condition, rule.Threshold, value)
}

// NotifyTelegram 发送 Telegram 通知（预留，当前未实现）
func (n *Notifier) NotifyTelegram(_ context.Context, _ *model.AlertRule, _, _ string, _ float64, _ string) error {
	return nil
}
