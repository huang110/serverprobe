package service

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/collector"
	"github.com/huang110/serverprobe/agent/internal/config"
	"github.com/huang110/serverprobe/agent/pkg/logger"
	pb "github.com/huang110/serverprobe/agent/proto"
)

const (
	// defaultHeartbeatInterval 配置缺失时使用的心跳间隔
	defaultHeartbeatInterval = 5 * time.Second

	// initialHeartbeatBackoff 心跳失败后的首次重试间隔
	initialHeartbeatBackoff = time.Second

	// maxHeartbeatBackoff 心跳失败后的最大重试间隔
	maxHeartbeatBackoff = 60 * time.Second
)

// HeartbeatService 心跳服务。
// 定时采集 CPU、内存、磁盘、网络和系统指标，签名后上报 Server，
// 并处理 Token 轮换、心跳间隔调整和服务端配置更新。
// 服务实例只能启动一次，Stop 之后不可重启。
type HeartbeatService struct {
	grpcClient *client.GRPCClient
	config     *config.Config
	loader     *config.Loader

	cpuCollector  *collector.CPUCollector
	memCollector  *collector.MemoryCollector
	diskCollector *collector.DiskCollector
	netCollector  *collector.NetworkCollector
	sysCollector  *collector.SystemCollector

	// latencyCollector 多目标 TCP Ping 延迟采集器，未配置目标时为 nil
	latencyCollector *collector.LatencyCollector

	mu        sync.Mutex
	interval  time.Duration
	failures  int
	started   bool
	stopped   bool
	stopCh    chan struct{}
	doneCh    chan struct{}
	startTime time.Time
}

// NewHeartbeatService 创建心跳服务并初始化所有采集器
func NewHeartbeatService(grpcClient *client.GRPCClient, cfg *config.Config, loader *config.Loader) *HeartbeatService {
	service := &HeartbeatService{
		grpcClient: grpcClient,
		config:     cfg,
		loader:     loader,
		startTime:  time.Now(),
		interval:   defaultHeartbeatInterval,
	}

	if cfg != nil {
		service.interval = cfg.GetHeartbeatInterval()
		if service.interval <= 0 {
			service.interval = defaultHeartbeatInterval
		}
		service.cpuCollector = collector.NewCPUCollector()
		service.memCollector = collector.NewMemoryCollector()
		service.diskCollector = collector.NewDiskCollector(
			collector.WithDiskMountPoints(cfg.Monitor.DiskMounts),
			collector.WithExcludedFSTypes(cfg.Monitor.ExcludeFSTypes),
		)
		service.netCollector = collector.NewNetworkCollector(
			collector.WithNetworkInterfaces(cfg.Monitor.NetworkInterfaces),
		)
		service.sysCollector = collector.NewSystemCollector()
		if len(cfg.IPQuality.LatencyTargets) > 0 {
			service.latencyCollector = collector.NewLatencyCollector(
				cfg.IPQuality.LatencyTargets,
				collector.WithLatencyCheckInterval(cfg.GetIPQualityInterval()),
			)
		}
	} else {
		service.cpuCollector = collector.NewCPUCollector()
		service.memCollector = collector.NewMemoryCollector()
		service.diskCollector = collector.NewDiskCollector()
		service.netCollector = collector.NewNetworkCollector()
		service.sysCollector = collector.NewSystemCollector()
	}

	return service
}

// Start 启动心跳循环。首次心跳会立即发送，之后按配置的间隔执行。
func (h *HeartbeatService) Start() error {
	if h.config == nil {
		return errors.New("config is required")
	}
	if h.loader == nil {
		return errors.New("config loader is required")
	}
	if h.grpcClient == nil {
		return errors.New("gRPC client is required")
	}
	if h.config.Agent.ID == "" || h.config.Agent.Token == "" {
		return errors.New("agent is not registered")
	}

	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return errors.New("heartbeat service has been stopped and cannot be restarted")
	}
	if h.started {
		h.mu.Unlock()
		return errors.New("heartbeat service already started")
	}

	h.stopCh = make(chan struct{})
	h.doneCh = make(chan struct{})
	h.started = true
	h.startTime = time.Now()
	h.failures = 0
	// 注册流程或服务端配置可能已经修改了心跳间隔，启动时以最新配置为准
	if configInterval := h.config.GetHeartbeatInterval(); configInterval > 0 {
		h.interval = configInterval
	}
	interval := h.interval
	h.mu.Unlock()

	go h.run()

	logger.Infof("Heartbeat service started, interval: %s", interval)
	return nil
}

// Stop 停止心跳循环，并等待正在执行的心跳结束。
// 可安全重复调用；未启动时调用不会产生副作用。
func (h *HeartbeatService) Stop() {
	h.mu.Lock()
	if !h.started || h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true
	stopCh := h.stopCh
	doneCh := h.doneCh
	h.mu.Unlock()

	close(stopCh)
	<-doneCh
	logger.Info("Heartbeat service stopped")
}

// IsRunning 返回心跳服务是否正在运行
func (h *HeartbeatService) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started && !h.stopped
}

// run 心跳主循环。
// 使用 ticker 等待下一次心跳，失败时采用指数退避，成功后恢复配置间隔。
func (h *HeartbeatService) run() {
	defer close(h.doneCh)

	ticker := time.NewTicker(h.currentInterval())
	defer ticker.Stop()

	for {
		// 停止信号优先于首次心跳，避免无意义的采集和请求
		select {
		case <-h.stopCh:
			return
		default:
		}

		h.performHeartbeat()

		delay := h.nextHeartbeatDelay()
		ticker.Reset(delay)
		logger.Debugf("Next heartbeat scheduled in %s", delay)

		select {
		case <-ticker.C:
		case <-h.stopCh:
			return
		}
	}
}

// performHeartbeat 执行一次心跳并更新失败计数
func (h *HeartbeatService) performHeartbeat() {
	if err := h.sendHeartbeat(); err != nil {
		failures := h.recordFailure()
		logger.Errorf("Heartbeat failed (consecutive failures: %d): %v", failures, err)
		return
	}
	h.recordSuccess()
}

// sendHeartbeat 采集指标、生成签名并发送心跳
func (h *HeartbeatService) sendHeartbeat() error {
	if h.config == nil {
		return errors.New("config is required")
	}
	if h.grpcClient == nil {
		return errors.New("gRPC client is required")
	}
	if h.config.Agent.ID == "" || h.config.Agent.Token == "" {
		return errors.New("agent is not registered")
	}

	metrics := h.collectMetrics()

	timestamp := time.Now().Unix()
	signature := client.GenerateSignature(h.config.Agent.ID, timestamp, h.config.Agent.Token)

	request := &pb.HeartbeatRequest{
		AgentId:        h.config.Agent.ID,
		Timestamp:      timestamp,
		Signature:      signature,
		Cpu:            convertCPUMetrics(metrics.cpu),
		Memory:         convertMemMetrics(metrics.memory),
		Disks:          convertDiskMetrics(metrics.disks),
		Networks:       convertNetMetrics(metrics.networks),
		System:         convertSysMetrics(metrics.system),
		NetworkQuality: convertLatencyMetrics(metrics.latency, metrics.latencyCheckedAt),
		AgentStatus:    h.buildAgentStatus(metrics.cpu, metrics.memory),
	}

	response, err := h.grpcClient.Heartbeat(request)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	if response == nil {
		return errors.New("empty heartbeat response from server")
	}
	if !response.Success {
		return fmt.Errorf("heartbeat rejected: %s", response.Message)
	}

	configChanged := false

	// Token 轮换：先更新内存，确保后续心跳立即使用新 Token
	if response.NewToken != "" {
		h.config.Agent.Token = response.NewToken
		h.config.Agent.TokenExpiresAt = response.NewTokenExpiresAt
		configChanged = true
		logger.Infof("Agent token rotated, expires at %d", response.NewTokenExpiresAt)
	}

	// 服务端动态调整心跳间隔
	if response.NextHeartbeatInterval > 0 {
		h.applyHeartbeatInterval(response.NextHeartbeatInterval)
		configChanged = true
	}

	// 服务端下发配置更新
	if h.applyConfigUpdate(response.ConfigUpdate) {
		configChanged = true
	}

	// 注册后的配置变更统一持久化，避免多次写文件
	if configChanged && h.loader != nil {
		if err := h.loader.Save(h.config); err != nil {
			logger.Errorf("Failed to persist updated agent config: %v", err)
		}
	}

	logger.Debugf("Heartbeat sent successfully, agent_id: %s", h.config.Agent.ID)
	return nil
}

// collectedMetrics 一次采集得到的全部指标
type collectedMetrics struct {
	cpu      *collector.CPUMetrics
	memory   *collector.MemoryMetrics
	disks    []collector.DiskMetrics
	networks []collector.NetworkMetrics
	system   *collector.SystemMetrics

	latency          []collector.LatencyResult
	latencyCheckedAt int64
}

// collectMetrics 并发执行所有采集器，避免 CPU 采集阻塞其他指标
func (h *HeartbeatService) collectMetrics() *collectedMetrics {
	metrics := &collectedMetrics{}
	var wg sync.WaitGroup

	wg.Add(5)

	go func() {
		defer wg.Done()
		var err error
		metrics.cpu, err = h.cpuCollector.Collect()
		if err != nil {
			logger.Errorf("Failed to collect CPU metrics: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		metrics.memory, err = h.memCollector.Collect()
		if err != nil {
			logger.Errorf("Failed to collect memory metrics: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		metrics.disks, err = h.diskCollector.Collect()
		if err != nil {
			logger.Errorf("Failed to collect disk metrics: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		metrics.networks, err = h.netCollector.Collect()
		if err != nil {
			logger.Errorf("Failed to collect network metrics: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		metrics.system, err = h.sysCollector.Collect()
		if err != nil {
			logger.Errorf("Failed to collect system metrics: %v", err)
		}
	}()

	if h.latencyCollector != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.latency = h.latencyCollector.Collect()
			metrics.latencyCheckedAt = h.latencyCollector.LastCheckTime()
		}()
	}

	wg.Wait()
	return metrics
}

// buildAgentStatus 构造 Agent 自身状态
func (h *HeartbeatService) buildAgentStatus(cpu *collector.CPUMetrics, memory *collector.MemoryMetrics) *pb.AgentStatus {
	h.mu.Lock()
	startTime := h.startTime
	h.mu.Unlock()

	var uptime int64
	if !startTime.IsZero() {
		uptime = int64(time.Since(startTime).Seconds())
	}

	status := &pb.AgentStatus{
		Goroutines: int64(runtime.NumGoroutine()),
		Uptime:     uptime,
		Version:    config.Version,
	}
	if h.config != nil {
		status.LiteMode = h.config.Agent.LiteMode
	}
	if cpu != nil {
		status.CpuUsage = cpu.UsagePercent
	}
	if memory != nil {
		status.MemoryUsage = memory.Used
	}
	return status
}

// applyHeartbeatInterval 应用新的心跳间隔并同步到内存配置
func (h *HeartbeatService) applyHeartbeatInterval(seconds int32) {
	if seconds <= 0 {
		return
	}

	newInterval := time.Duration(seconds) * time.Second

	h.mu.Lock()
	previous := h.interval
	h.interval = newInterval
	h.mu.Unlock()

	if h.config != nil {
		h.config.Monitor.HeartbeatInterval = int(seconds)
	}

	if previous != newInterval {
		logger.Infof("Heartbeat interval changed from %s to %s", previous, newInterval)
	}
}

// applyConfigUpdate 应用服务端下发的配置更新，返回是否有变更
func (h *HeartbeatService) applyConfigUpdate(update *pb.ConfigUpdate) bool {
	if update == nil || !update.HasUpdate {
		return false
	}

	changed := false

	if update.HeartbeatInterval > 0 {
		h.applyHeartbeatInterval(update.HeartbeatInterval)
		changed = true
	}

	if len(update.Labels) > 0 && h.config != nil {
		if h.config.Labels == nil {
			h.config.Labels = make(map[string]string, len(update.Labels))
		}
		for key, value := range update.Labels {
			h.config.Labels[key] = value
		}
		changed = true
	}

	if h.config != nil {
		if update.EnableIpQuality {
			if !h.config.IPQuality.Enabled {
				h.config.IPQuality.Enabled = true
				changed = true
			}
		} else if h.config.IPQuality.Enabled && len(h.config.IPQuality.LatencyTargets) == 0 {
			// 本地未配置延迟目标时才跟随服务端关闭，
			// 避免服务端下发的默认 false 覆盖本地已开启的 TCP Ping 采集。
			h.config.IPQuality.Enabled = false
			changed = true
		}
	}

	return changed
}

// currentInterval 返回当前使用的心跳间隔
func (h *HeartbeatService) currentInterval() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.interval > 0 {
		return h.interval
	}
	if h.config != nil {
		return h.config.GetHeartbeatInterval()
	}
	return defaultHeartbeatInterval
}

// nextHeartbeatDelay 返回下一次心跳的等待时间。
// 连续失败时使用指数退避（1s、2s、4s...，最大 60s）。
func (h *HeartbeatService) nextHeartbeatDelay() time.Duration {
	h.mu.Lock()
	failures := h.failures
	h.mu.Unlock()

	if failures > 0 {
		return heartbeatBackoff(failures)
	}
	return h.currentInterval()
}

// recordFailure 增加连续失败计数，返回最新值
func (h *HeartbeatService) recordFailure() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.failures++
	return h.failures
}

// recordSuccess 重置连续失败计数
func (h *HeartbeatService) recordSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failures = 0
}

// heartbeatBackoff 计算心跳失败后的指数退避时间
func heartbeatBackoff(failures int) time.Duration {
	if failures <= 0 {
		return 0
	}

	backoff := initialHeartbeatBackoff
	for i := 1; i < failures; i++ {
		if backoff >= maxHeartbeatBackoff/2 {
			return maxHeartbeatBackoff
		}
		backoff *= 2
	}
	if backoff > maxHeartbeatBackoff {
		return maxHeartbeatBackoff
	}
	return backoff
}

// convertCPUMetrics 转换 CPU 指标
func convertCPUMetrics(metrics *collector.CPUMetrics) *pb.CPUMetrics {
	if metrics == nil {
		return nil
	}
	return &pb.CPUMetrics{
		UsagePercent:  metrics.UsagePercent,
		UserPercent:   metrics.UserPercent,
		SystemPercent: metrics.SystemPercent,
		IdlePercent:   metrics.IdlePercent,
		IowaitPercent: metrics.IowaitPercent,
		StealPercent:  metrics.StealPercent,
		PerCoreUsage:  metrics.PerCoreUsage,
	}
}

// convertMemMetrics 转换内存指标
func convertMemMetrics(metrics *collector.MemoryMetrics) *pb.MemoryMetrics {
	if metrics == nil {
		return nil
	}
	return &pb.MemoryMetrics{
		Total:            metrics.Total,
		Used:             metrics.Used,
		Free:             metrics.Free,
		Available:        metrics.Available,
		UsagePercent:     metrics.UsagePercent,
		SwapTotal:        metrics.SwapTotal,
		SwapUsed:         metrics.SwapUsed,
		SwapFree:         metrics.SwapFree,
		SwapUsagePercent: metrics.SwapUsagePercent,
		Buffers:          metrics.Buffers,
		Cached:           metrics.Cached,
		Shared:           metrics.Shared,
	}
}

// convertDiskMetrics 转换磁盘指标
func convertDiskMetrics(metrics []collector.DiskMetrics) []*pb.DiskMetrics {
	if len(metrics) == 0 {
		return nil
	}

	result := make([]*pb.DiskMetrics, 0, len(metrics))
	for _, metric := range metrics {
		result = append(result, &pb.DiskMetrics{
			Device:            metric.Device,
			MountPoint:        metric.MountPoint,
			FsType:            metric.FsType,
			Total:             metric.Total,
			Used:              metric.Used,
			Free:              metric.Free,
			UsagePercent:      metric.UsagePercent,
			InodeTotal:        metric.InodeTotal,
			InodeUsed:         metric.InodeUsed,
			InodeUsagePercent: metric.InodeUsagePercent,
			ReadBytesPerSec:   metric.ReadBytesPerSec,
			WriteBytesPerSec:  metric.WriteBytesPerSec,
			ReadIops:          metric.ReadIOPS,
			WriteIops:         metric.WriteIOPS,
			IoUtilPercent:     metric.IOUtilPercent,
		})
	}
	return result
}

// convertNetMetrics 转换网络指标
func convertNetMetrics(metrics []collector.NetworkMetrics) []*pb.NetworkMetrics {
	if len(metrics) == 0 {
		return nil
	}

	result := make([]*pb.NetworkMetrics, 0, len(metrics))
	for _, metric := range metrics {
		result = append(result, &pb.NetworkMetrics{
			Interface:     metric.Interface,
			IpAddress:     metric.IPAddress,
			MacAddress:    metric.MACAddress,
			RxBytesPerSec: metric.RxBytesPerSec,
			TxBytesPerSec: metric.TxBytesPerSec,
			RxBytesTotal:  metric.RxBytesTotal,
			TxBytesTotal:  metric.TxBytesTotal,
			RxPackets:     metric.RxPackets,
			TxPackets:     metric.TxPackets,
			RxErrors:      metric.RxErrors,
			TxErrors:      metric.TxErrors,
			RxDropped:     metric.RxDropped,
			TxDropped:     metric.TxDropped,
		})
	}
	return result
}

// convertSysMetrics 转换系统指标
func convertSysMetrics(metrics *collector.SystemMetrics) *pb.SystemMetrics {
	if metrics == nil {
		return nil
	}
	return &pb.SystemMetrics{
		Load1:          metrics.Load1,
		Load5:          metrics.Load5,
		Load15:         metrics.Load15,
		Uptime:         metrics.Uptime,
		ProcessCount:   metrics.ProcessCount,
		TcpEstablished: metrics.TcpEstablished,
		TcpTimeWait:    metrics.TcpTimeWait,
		TcpCloseWait:   metrics.TcpCloseWait,
		UdpConnections: metrics.UdpConnections,
	}
}

// convertLatencyMetrics 转换 TCP Ping 延迟指标
func convertLatencyMetrics(results []collector.LatencyResult, checkedAt int64) *pb.NetworkQualityMetrics {
	if len(results) == 0 {
		return nil
	}

	tests := make([]*pb.LatencyTest, 0, len(results))
	for _, result := range results {
		tests = append(tests, &pb.LatencyTest{
			Target:     result.Target,
			TargetName: result.TargetName,
			LatencyMs:  result.LatencyMs,
			JitterMs:   result.JitterMs,
			PacketLoss: result.PacketLoss,
		})
	}

	return &pb.NetworkQualityMetrics{
		LatencyTests:  tests,
		LastCheckTime: checkedAt,
	}
}
