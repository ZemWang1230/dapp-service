package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"timelocker-backend/docs"
	abiHandler "timelocker-backend/internal/api/abi"
	authHandler "timelocker-backend/internal/api/auth"
	chainHandler "timelocker-backend/internal/api/chain"
	emailHandler "timelocker-backend/internal/api/email"
	flowHandler "timelocker-backend/internal/api/flow"
	goldskyHandler "timelocker-backend/internal/api/goldsky"
	notificationHandler "timelocker-backend/internal/api/notification"
	publicHandler "timelocker-backend/internal/api/public"
	timelockHandler "timelocker-backend/internal/api/timelock"

	"timelocker-backend/internal/config"
	abiRepo "timelocker-backend/internal/repository/abi"
	chainRepo "timelocker-backend/internal/repository/chain"
	emailRepo "timelocker-backend/internal/repository/email"
	goldskyRepo "timelocker-backend/internal/repository/goldsky"
	notificationRepo "timelocker-backend/internal/repository/notification"
	publicRepo "timelocker-backend/internal/repository/public"
	safeRepo "timelocker-backend/internal/repository/safe"
	timelockRepo "timelocker-backend/internal/repository/timelock"

	userRepo "timelocker-backend/internal/repository/user"
	abiService "timelocker-backend/internal/service/abi"
	authService "timelocker-backend/internal/service/auth"
	chainService "timelocker-backend/internal/service/chain"
	emailService "timelocker-backend/internal/service/email"
	flowService "timelocker-backend/internal/service/flow"
	goldskyService "timelocker-backend/internal/service/goldsky"
	notificationService "timelocker-backend/internal/service/notification"
	publicService "timelocker-backend/internal/service/public"
	scannerService "timelocker-backend/internal/service/scanner"
	timelockService "timelocker-backend/internal/service/timelock"

	"timelocker-backend/pkg/database"

	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Timelock Backend API
// @version 1.0
// @description Timelock Backend API
// @host localhost:8080
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	logger.Init(logger.DefaultConfig())
	logger.Info("Starting Timelock Backend v1.0.0")

	// 创建根context和WaitGroup用于协调关闭
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 1. 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config: ", err)
		os.Exit(1)
	}

	// 2. 连接数据库
	db, err := database.NewPostgresConnection(&cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database: ", err)
		os.Exit(1)
	}

	// 设置logger数据库写入器，使错误日志可以写入数据库
	logger.SetDB(db)

	// 3. 连接Redis
	// redisClient, err := database.NewRedisConnection(&cfg.Redis)
	// if err != nil {
	// 	logger.Error("Failed to connect to Redis: ", err)
	// 	os.Exit(1)
	// }

	// 4. 初始化仓库层
	userRepository := userRepo.NewRepository(db)
	chainRepository := chainRepo.NewRepository(db)
	abiRepository := abiRepo.NewRepository(db)
	timelockRepository := timelockRepo.NewRepository(db)
	emailRepository := emailRepo.NewEmailRepository(db)
	notificationRepository := notificationRepo.NewRepository(db)
	safeRepository := safeRepo.NewRepository(db)

	// Goldsky Flow 仓库
	goldskyFlowRepository := goldskyRepo.NewFlowRepository(db)

	// 公共数据仓库
	publicRepository := publicRepo.NewRepository(db)

	// 5. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 6. 初始化服务层
	abiSvc := abiService.NewService(abiRepository)
	chainSvc := chainService.NewService(chainRepository)

	// 初始化 email 和 notification 服务（使用 Goldsky Flow Repository）
	emailSvc := emailService.NewEmailService(emailRepository, chainRepository, timelockRepository, goldskyFlowRepository, cfg)
	notificationSvc := notificationService.NewNotificationService(notificationRepository, chainRepository, timelockRepository, goldskyFlowRepository, cfg)

	// 初始化 Goldsky 服务
	goldskySvc := goldskyService.NewGoldskyService(
		chainRepository,
		timelockRepository,
		goldskyFlowRepository,
		publicRepository,
		emailSvc,
		notificationSvc,
	)

	// 初始化公共服务
	publicSvc := publicService.NewService(publicRepository)

	// 初始化 Goldsky Webhook Processor
	goldskyProcessor := goldskyService.NewWebhookProcessor(
		timelockRepository,
		goldskyFlowRepository,
		emailSvc,
		notificationSvc,
	)

	// 初始化 Flow 服务
	flowSvc := flowService.NewFlowService(goldskyFlowRepository, chainRepository, goldskySvc)

	// 7. 设置Gin和路由
	gin.SetMode(cfg.Server.Mode)
	router := gin.Default()

	// 8. 添加CORS中间件
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	// 9. 创建API路由组
	v1 := router.Group("/api/v1")
	{
		chainHandler := chainHandler.NewHandler(chainSvc)
		chainHandler.RegisterRoutes(v1)

		publicHdl := publicHandler.NewHandler(publicSvc)
		publicHdl.RegisterRoutes(v1)

	}

	// 10. Swagger API文档端点
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	docs.SwaggerInfo.Title = "Timelock Backend API v1.0"
	docs.SwaggerInfo.Description = "Timelock Backend API"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// 健康检查端点
	router.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 11. 启动 RPC 管理器（auth 和 timelock 服务需要）
	rpcManager := scannerService.NewRPCManager(cfg, chainRepository)
	if err := rpcManager.Start(ctx); err != nil {
		logger.Error("Failed to start RPC manager", err)
	} else {
		logger.Info("RPC Manager started successfully")
	}

	// 12. 启动 Goldsky 服务
	if err := goldskySvc.Start(); err != nil {
		logger.Error("Failed to start Goldsky service", err)
	} else {
		logger.Info("Goldsky service started successfully")
	}

	// 13. 初始化需要 RPC 的服务和处理器
	authSvc := authService.NewService(userRepository, safeRepository, rpcManager, jwtManager)
	timelockSvc := timelockService.NewService(timelockRepository, chainRepository, rpcManager, goldskySvc)

	// 14. 初始化处理器并注册路由
	authHandler := authHandler.NewHandler(authSvc)
	authHandler.RegisterRoutes(v1)

	abiHandler := abiHandler.NewHandler(abiSvc, authSvc)
	abiHandler.RegisterRoutes(v1)

	timelockHandler := timelockHandler.NewHandler(timelockSvc, authSvc)
	timelockHandler.RegisterRoutes(v1)

	emailHdl := emailHandler.NewEmailHandler(emailSvc, authSvc)
	emailHdl.RegisterRoutes(v1)

	flowHdl := flowHandler.NewFlowHandler(flowSvc, authSvc)
	flowHdl.RegisterRoutes(v1)

	notificationHdl := notificationHandler.NewNotificationHandler(notificationSvc, authSvc)
	notificationHdl.RegisterRoutes(v1)

	goldskyHdl := goldskyHandler.NewWebhookHandler(goldskyProcessor, chainRepository)
	goldskyHdl.RegisterRoutes(v1)

	// goldskySyncHdl := goldskyHandler.NewSyncHandler(goldskySvc)
	// goldskySyncHdl.RegisterRoutes(v1)

	// 15. 启动定时任务
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logger.Info("Timelock refresh task stopped")

		ticker := time.NewTicker(2 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logger.Info("Starting scheduled timelock data refresh")
				if err := timelockSvc.RefreshAllTimeLockData(ctx); err != nil {
					logger.Error("Failed to refresh timelock data", err)
				} else {
					logger.Info("Scheduled timelock data refresh completed successfully")
				}
			}
		}
	}()

	// 16. 启动邮箱验证码清理定时任务
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logger.Info("Email verification code cleanup task stopped")

		ticker := time.NewTicker(30 * time.Minute) // 每30分钟清理一次过期验证码
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logger.Info("Starting scheduled email verification code cleanup")
				if err := emailSvc.CleanExpiredCodes(ctx); err != nil {
					logger.Error("Failed to clean expired verification codes", err)
				} else {
					logger.Info("Scheduled email verification code cleanup completed successfully")
				}
			}
		}
	}()

	// 17. 启动HTTP服务器
	addr := ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting server on ", "address", addr)
		logger.Info("Swagger documentation available at: http://localhost:" + cfg.Server.Port + "/swagger/index.html")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error: ", err)
			cancel() // 通知其他组件关闭
		}
	}()

	// 18. 等待关闭信号
	<-sigCh
	logger.Info("Received shutdown signal, starting graceful shutdown...")

	// 19. 开始优雅关闭（逆序关闭）

	// Step 1: 停止HTTP服务器（最后启动的最先关闭）
	logger.Info("Stopping HTTP server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error: ", err)
	} else {
		logger.Info("HTTP server stopped")
	}
	shutdownCancel()

	// Step 2: 取消context，通知所有服务停止
	logger.Info("Cancelling context to stop all services...")
	cancel()

	// Step 3: 停止 Goldsky 服务
	logger.Info("Stopping Goldsky service...")
	goldskySvc.Stop()

	// Step 4: 停止 RPC 管理器
	logger.Info("Stopping RPC manager...")
	rpcManager.Stop()

	// Step 5: 等待所有goroutine结束
	logger.Info("Waiting for all goroutines to finish...")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All services stopped gracefully")
	case <-time.After(15 * time.Second):
		logger.Error("Timeout waiting for services to stop, forcing exit", nil)
	}
}
