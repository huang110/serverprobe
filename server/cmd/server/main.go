package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/aggregator"
	"github.com/huang110/serverprobe/server/internal/alert"
	"github.com/huang110/serverprobe/server/internal/api/handler"
	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	grpchandler "github.com/huang110/serverprobe/server/internal/grpc"
	"github.com/huang110/serverprobe/server/internal/middleware"
	"github.com/huang110/serverprobe/server/internal/model"
	"github.com/huang110/serverprobe/server/internal/websocket"
)

var (
	configPath  = flag.String("config", "config.yaml", "配置文件路径")
	showVersion = flag.Bool("version", false, "显示版本信息")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("ServerProbe Server v%s\n", config.Version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer closeDB(db)

	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	redisClient, err := cache.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Fatalf("failed to init redis: %v", err)
	}
	defer func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			log.Printf("failed to close redis: %v", closeErr)
		}
	}()

	// WebSocket Hub 与事件广播器
	hub := websocket.NewHub()
	go hub.Run()
	broadcaster := websocket.NewBroadcaster(hub, redisClient)
	go broadcaster.Start()

	// gRPC Server
	grpcServer, err := startGRPCServer(cfg, db, redisClient)
	if err != nil {
		log.Fatalf("failed to start gRPC server: %v", err)
	}

	// HTTP Server
	httpServer := startHTTPServer(cfg, db, redisClient, hub)

	// 后台任务
	alertEngine := alert.NewEngine(db, redisClient, cfg)
	go alertEngine.Start()

	dataAggregator := aggregator.NewAggregatorWithOptions(db, cfg.GetAggregationInterval(), cfg.IncludeCurrentAggregationBucket())
	if cfg.Aggregation.Enabled {
		go dataAggregator.Start()
	}

	cleaner := aggregator.NewCleaner(db)
	go cleaner.Start()

	log.Printf("ServerProbe started: http=%s grpc=%s", cfg.GetHTTPAddress(), cfg.GetGRPCAddress())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// gRPC 优雅停止，最多等待 10 秒
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-ctx.Done():
		grpcServer.Stop()
	}

	alertEngine.Stop()
	dataAggregator.Stop()
	cleaner.Stop()
	broadcaster.Stop()
	hub.Stop()

	log.Println("server stopped")
}

// initDB 初始化数据库连接
func initDB(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "", "mysql":
		dialector = mysql.Open(cfg.Database.DSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.Database.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	maxOpen := cfg.Database.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 100
	}
	maxIdle := cfg.Database.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func closeDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		return
	}
	if closeErr := sqlDB.Close(); closeErr != nil {
		log.Printf("failed to close database: %v", closeErr)
	}
}

// startGRPCServer 启动 gRPC 服务器
func startGRPCServer(cfg *config.Config, db *gorm.DB, redisClient *cache.RedisClient) (*grpc.Server, error) {
	listener, err := net.Listen("tcp", cfg.GetGRPCAddress())
	if err != nil {
		return nil, err
	}

	server := grpc.NewServer()
	grpchandler.RegisterProbeServer(server, db, redisClient, cfg)

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
			log.Printf("gRPC server error: %v", serveErr)
		}
	}()

	return server, nil
}

// startHTTPServer 启动 HTTP 服务器
func startHTTPServer(cfg *config.Config, db *gorm.DB, redisClient *cache.RedisClient, hub *websocket.Hub) *http.Server {
	if cfg.Server.Mode == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.CORSMiddleware(cfg.CORS.AllowedOrigins))
	if cfg.RateLimit.Enabled {
		router.Use(middleware.RateLimitMiddleware(cfg.RateLimit.RequestsPerSecond))
	}

	registerRoutes(router, cfg, db, redisClient, hub)

	server := &http.Server{
		Addr:    cfg.GetHTTPAddress(),
		Handler: router,
	}

	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Printf("HTTP server error: %v", serveErr)
		}
	}()

	return server
}

// registerRoutes 注册 HTTP 路由
func registerRoutes(router *gin.Engine, cfg *config.Config, db *gorm.DB, redisClient *cache.RedisClient, hub *websocket.Hub) {
	healthHandler := handler.NewHealthHandler(db, redisClient)
	router.GET("/api/health", healthHandler.Health)
	router.GET("/api/health/ready", healthHandler.Ready)
	router.GET("/api/health/metrics", healthHandler.Metrics)

	authHandler := handler.NewAuthHandler(cfg)
	router.POST("/api/auth/login", authHandler.Login)
	router.GET("/ws", websocket.HandleWebSocket(hub, cfg.GetJWTSecret()))

	protected := router.Group("/api")
	protected.Use(middleware.JWTAuth(cfg.GetJWTSecret()))

	agentHandler := handler.NewAgentHandler(db, redisClient)
	protected.GET("/agents", agentHandler.ListAgents)
	protected.GET("/agents/:id", agentHandler.GetAgent)
	protected.PUT("/agents/:id", agentHandler.UpdateAgent)
	protected.DELETE("/agents/:id", agentHandler.DeleteAgent)

	metricsHandler := handler.NewMetricsHandler(db, redisClient)
	protected.GET("/agents/:id/metrics/realtime", metricsHandler.GetRealtimeMetrics)
	protected.GET("/agents/:id/metrics/history", metricsHandler.GetHistoryMetrics)

	alertHandler := handler.NewAlertHandler(db)
	protected.POST("/alert/rules", alertHandler.CreateRule)
	protected.GET("/alert/rules", alertHandler.ListRules)
	protected.GET("/alert/records", alertHandler.ListRecords)
}
