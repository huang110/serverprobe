package service

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/collector"
	"github.com/huang110/serverprobe/agent/internal/config"
	"github.com/huang110/serverprobe/agent/pkg/logger"
	pb "github.com/huang110/serverprobe/agent/proto"
)

const (
	// megabyte 字节到 MB 的换算系数
	megabyte = 1024 * 1024

	// publicIPLookupURL 公网 IP 查询地址
	publicIPLookupURL = "https://api.ipify.org"

	// publicIPLookupTimeout 公网 IP 查询超时时间
	publicIPLookupTimeout = 3 * time.Second

	// internalIPProbeAddress 用于探测默认路由的远端地址。
	// UDP Dial 不会真正发送数据包，仅用于获取本机出口 IP。
	internalIPProbeAddress = "8.8.8.8:80"

	// internalIPProbeTimeout 默认路由探测超时时间
	internalIPProbeTimeout = time.Second
)

// ErrRegistrationRejected 表示 Server 明确拒绝了注册请求（业务错误）。
// 网络错误和业务错误可通过 errors.Is 区分。
var ErrRegistrationRejected = errors.New("registration rejected")

// RegisterService Agent 注册服务。
// 负责首次注册、Token 过期后重新注册，并将服务端返回的凭据持久化到配置文件。
type RegisterService struct {
	grpcClient *client.GRPCClient
	config     *config.Config
	loader     *config.Loader

	// 允许替换 IP 探测逻辑，便于测试和特殊网络环境
	publicIPLookup   func() string
	internalIPLookup func() string
}

// NewRegisterService 创建注册服务
func NewRegisterService(grpcClient *client.GRPCClient, cfg *config.Config, loader *config.Loader) *RegisterService {
	return &RegisterService{
		grpcClient:       grpcClient,
		config:           cfg,
		loader:           loader,
		publicIPLookup:   getPublicIP,
		internalIPLookup: getInternalIP,
	}
}

// Register 执行注册流程。
// 已注册且 Token 未过期时直接返回；否则收集系统信息并向 Server 发起注册。
func (r *RegisterService) Register() error {
	if r.config == nil {
		return errors.New("config is required")
	}
	if r.loader == nil {
		return errors.New("config loader is required")
	}

	// 已注册且 Token 未过期，无需重复注册
	if r.config.IsRegistered() && !r.config.IsTokenExpired() {
		logger.Infof("Agent already registered, agent_id: %s", r.config.Agent.ID)
		return nil
	}

	if strings.TrimSpace(r.config.Server.RegisterSecret) == "" {
		return errors.New("register_secret is required for registration")
	}
	if r.grpcClient == nil {
		return errors.New("gRPC client is required for registration")
	}

	hostname, err := getHostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	internalIP := r.internalIP()
	publicIP := r.publicIP()
	if publicIP == "" {
		// 公网 IP 查询失败时使用内网 IP 兜底，服务端仍可通过连接获取真实来源 IP
		publicIP = internalIP
	}

	request := &pb.RegisterRequest{
		RegisterSecret: r.config.Server.RegisterSecret,
		Hostname:       hostname,
		Ip:             publicIP,
		InternalIp:     internalIP,
		Os:             runtime.GOOS,
		OsVersion:      getOSVersion(),
		KernelVersion:  getKernelVersion(),
		CpuCores:       getCPUCores(),
		MemoryTotal:    getMemoryTotal(),
		DiskTotal:      getDiskTotal(),
		Arch:           runtime.GOARCH,
		AgentVersion:   config.Version,
		Labels:         r.config.Labels,
		LiteMode:       r.config.Agent.LiteMode,
	}

	logger.Infof("Registering agent with server, hostname: %s, ip: %s", hostname, publicIP)

	response, err := r.grpcClient.Register(request)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	if response == nil {
		return errors.New("registration failed: empty response from server")
	}
	if !response.Success {
		return fmt.Errorf("%w: %s", ErrRegistrationRejected, response.Message)
	}
	if strings.TrimSpace(response.AgentId) == "" || strings.TrimSpace(response.Token) == "" {
		return errors.New("registration failed: server returned empty agent_id or token")
	}

	if err := r.loader.UpdateAgent(response.AgentId, response.Token, response.TokenExpiresAt); err != nil {
		return fmt.Errorf("failed to save registration: %w", err)
	}

	// 同步内存中的配置，后续心跳立即使用新的 Agent 凭据
	r.config.Agent.ID = response.AgentId
	r.config.Agent.Token = response.Token
	r.config.Agent.TokenExpiresAt = response.TokenExpiresAt

	// 服务端可以下发心跳间隔，注册完成后立即生效
	if response.HeartbeatInterval > 0 {
		r.config.Monitor.HeartbeatInterval = int(response.HeartbeatInterval)
		if err := r.loader.UpdateHeartbeatInterval(response.HeartbeatInterval); err != nil {
			logger.Warnf("Failed to persist heartbeat interval: %v", err)
		}
	}

	logger.Infof("Registration successful, agent_id: %s", response.AgentId)
	return nil
}

// publicIP 获取公网 IP，失败时返回空字符串
func (r *RegisterService) publicIP() string {
	if r.publicIPLookup == nil {
		return getPublicIP()
	}
	return r.publicIPLookup()
}

// internalIP 获取内网 IP，失败时返回空字符串
func (r *RegisterService) internalIP() string {
	if r.internalIPLookup == nil {
		return getInternalIP()
	}
	return r.internalIPLookup()
}

// getHostname 获取主机名
func getHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("os.Hostname failed: %w", err)
	}

	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return "", errors.New("hostname is empty")
	}
	return hostname, nil
}

// getPublicIP 通过公共 HTTP 服务查询公网 IP。
// 查询失败不会中断注册，调用方会回退到内网 IP。
func getPublicIP() string {
	httpClient := &http.Client{Timeout: publicIPLookupTimeout}
	response, err := httpClient.Get(publicIPLookupURL)
	if err != nil {
		logger.Warnf("Failed to query public IP: %v", err)
		return ""
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			logger.Debugf("Failed to close public IP response body: %v", closeErr)
		}
	}()

	if response.StatusCode != http.StatusOK {
		logger.Warnf("Failed to query public IP: unexpected status %d", response.StatusCode)
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, 64))
	if err != nil {
		logger.Warnf("Failed to read public IP response: %v", err)
		return ""
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		logger.Warnf("Public IP service returned an invalid address: %q", ip)
		return ""
	}
	return ip
}

// getInternalIP 获取本机内网 IP。
// 优先通过默认路由探测出口地址，失败后遍历网卡地址。
func getInternalIP() string {
	conn, err := net.DialTimeout("udp", internalIPProbeAddress, internalIPProbeTimeout)
	if err == nil {
		localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
		closeErr := conn.Close()
		if closeErr != nil {
			logger.Debugf("Failed to close IP probe connection: %v", closeErr)
		}
		if ok && localAddr != nil && localAddr.IP != nil && !localAddr.IP.IsLoopback() {
			return localAddr.IP.String()
		}
	} else {
		logger.Debugf("Failed to probe default route: %v", err)
	}

	return getInternalIPFromInterfaces()
}

// getInternalIPFromInterfaces 遍历网卡地址，优先返回私有 IPv4
func getInternalIPFromInterfaces() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Warnf("Failed to list network interface addresses: %v", err)
		return ""
	}

	var fallbackIPv4 string
	var fallbackIPv6 string

	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}

		if ip.To4() != nil {
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallbackIPv4 == "" {
				fallbackIPv4 = ip.String()
			}
			continue
		}
		if fallbackIPv6 == "" {
			fallbackIPv6 = ip.String()
		}
	}

	if fallbackIPv4 != "" {
		return fallbackIPv4
	}
	return fallbackIPv6
}

// getOSVersion 获取操作系统版本描述
func getOSVersion() string {
	hostInfo, err := host.Info()
	if err != nil {
		logger.Warnf("Failed to get OS information: %v", err)
		return runtime.GOOS
	}
	return formatOSVersion(hostInfo.Platform, hostInfo.PlatformVersion)
}

// formatOSVersion 拼接系统平台和版本号
func formatOSVersion(platform, version string) string {
	platform = strings.TrimSpace(platform)
	version = strings.TrimSpace(version)

	switch {
	case platform == "" && version == "":
		return runtime.GOOS
	case platform == "":
		return version
	case version == "":
		return platform
	default:
		return platform + " " + version
	}
}

// getKernelVersion 获取内核版本
func getKernelVersion() string {
	hostInfo, err := host.Info()
	if err != nil {
		logger.Warnf("Failed to get kernel information: %v", err)
		return ""
	}
	return hostInfo.KernelVersion
}

// getCPUCores 获取 CPU 核心数
func getCPUCores() int32 {
	cores, err := collector.GetCPUCores()
	if err != nil {
		logger.Warnf("Failed to get CPU cores: %v", err)
		return int32(runtime.NumCPU())
	}
	return cores
}

// getMemoryTotal 获取物理内存总量（MB）
func getMemoryTotal() int64 {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		logger.Warnf("Failed to get memory information: %v", err)
		return 0
	}
	return int64(vmStat.Total / megabyte)
}

// getDiskTotal 获取所有物理磁盘总容量（MB）
func getDiskTotal() int64 {
	metrics, err := collector.NewDiskCollector().Collect()
	if err != nil {
		logger.Warnf("Failed to get disk information: %v", err)
		return 0
	}

	var total int64
	for _, metric := range metrics {
		total += metric.Total
	}
	return total
}
