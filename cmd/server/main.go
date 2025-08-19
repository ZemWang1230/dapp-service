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
	sponsorHandler "timelocker-backend/internal/api/sponsor"
	timelockHandler "timelocker-backend/internal/api/timelock"

	"timelocker-backend/internal/config"
	abiRepo "timelocker-backend/internal/repository/abi"
	chainRepo "timelocker-backend/internal/repository/chain"
	emailRepo "timelocker-backend/internal/repository/email"

	scannerRepo "timelocker-backend/internal/repository/scanner"
	sponsorRepo "timelocker-backend/internal/repository/sponsor"
	timelockRepo "timelocker-backend/internal/repository/timelock"

	userRepo "timelocker-backend/internal/repository/user"
	abiService "timelocker-backend/internal/service/abi"
	authService "timelocker-backend/internal/service/auth"
	chainService "timelocker-backend/internal/service/chain"
	emailService "timelocker-backend/internal/service/email"
	flowService "timelocker-backend/internal/service/flow"
	scannerService "timelocker-backend/internal/service/scanner"
	sponsorService "timelocker-backend/internal/service/sponsor"
	timelockService "timelocker-backend/internal/service/timelock"

	"timelocker-backend/pkg/database"

	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title TimeLocker Backend API
// @version 1.0
// @description TimeLocker Backend API
// @host localhost:8080
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// updateAllScannersStatusToPaused 更新所有扫链器状态为暂停
func updateAllScannersStatusToPaused(ctx context.Context, progressRepo scannerRepo.ProgressRepository) error {
	// 批量更新所有运行中的扫描器状态为 paused
	return progressRepo.UpdateAllRunningScannersToPaused(ctx)
}

func main() {
	logger.Init(logger.DefaultConfig())
	logger.Info("Starting TimeLocker Backend v2.0.0")

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
	sponsorRepository := sponsorRepo.NewRepository(db)
	timelockRepository := timelockRepo.NewRepository(db)
	emailRepository := emailRepo.NewEmailRepository(db)

	// 扫链相关仓库
	progressRepository := scannerRepo.NewProgressRepository(db)
	transactionRepository := scannerRepo.NewTransactionRepository(db)
	flowRepository := scannerRepo.NewFlowRepository(db)

	// 5. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 6. 初始化服务层
	authSvc := authService.NewService(userRepository, jwtManager)
	abiSvc := abiService.NewService(abiRepository)
	chainSvc := chainService.NewService(chainRepository)
	sponsorSvc := sponsorService.NewService(sponsorRepository)
	emailSvc := emailService.NewEmailService(emailRepository, chainRepository, cfg)
	flowSvc := flowService.NewFlowService(flowRepository, timelockRepository)

	// 7. 初始化处理器
	authHandler := authHandler.NewHandler(authSvc)
	abiHandler := abiHandler.NewHandler(abiSvc, authSvc)
	chainHandler := chainHandler.NewHandler(chainSvc)
	sponsorHdl := sponsorHandler.NewHandler(sponsorSvc)
	emailHdl := emailHandler.NewEmailHandler(emailSvc, authSvc)
	flowHdl := flowHandler.NewFlowHandler(flowSvc, authSvc)

	// 8. 设置Gin和路由
	gin.SetMode(cfg.Server.Mode)
	router := gin.Default()

	// 9. 添加CORS中间件
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

	// 10. 注册API路由
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
		abiHandler.RegisterRoutes(v1)
		chainHandler.RegisterRoutes(v1)
		sponsorHdl.RegisterRoutes(v1)
		emailHdl.RegisterRoutes(v1)
		flowHdl.RegisterRoutes(v1)
	}

	// 11. Swagger API文档端点
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	docs.SwaggerInfo.Title = "TimeLocker Backend API v1.0"
	docs.SwaggerInfo.Description = "TimeLocker Backend API"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 12. 启动RPC管理器
	rpcManager := scannerService.NewRPCManager(cfg, chainRepository)
	if err := rpcManager.Start(ctx); err != nil {
		logger.Error("Failed to start RPC manager", err)
	} else {
		logger.Info("RPC Manager started successfully")
	}

	// 13. 启动扫链管理器
	scannerManager := scannerService.NewManager(
		cfg,
		chainRepository,
		timelockRepository,
		progressRepository,
		transactionRepository,
		flowRepository,
		rpcManager,
		emailSvc,
	)
	if err := scannerManager.Start(ctx); err != nil {
		logger.Error("Failed to start scanner manager", err)
	} else {
		logger.Info("Scanner Manager started successfully")
	}

	// 14. 初始化timelock服务（依赖RPC管理器）
	timelockSvc := timelockService.NewService(timelockRepository, chainRepository, rpcManager, cfg)
	timelockHandler := timelockHandler.NewHandler(timelockSvc, authSvc)
	timelockHandler.RegisterRoutes(v1)

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

	// Step 2: 取消context，通知所有扫链组件停止
	logger.Info("Cancelling context to stop all scanner services...")
	cancel()

	// Step 3: 停止扫链管理器（会自动更新状态为paused）
	logger.Info("Stopping scanner manager...")
	scannerManager.Stop()

	// Step 4: 停止RPC管理器
	logger.Info("Stopping RPC manager...")
	rpcManager.Stop()

	// Step 5: 确保所有扫链器状态已更新为paused（兜底保护）
	logger.Info("Ensuring all scanner status updated to paused...")
	shutdownCtx, shutdownCancel = context.WithTimeout(context.Background(), 3*time.Second)
	if err := updateAllScannersStatusToPaused(shutdownCtx, progressRepository); err != nil {
		logger.Error("Failed to ensure scanner status paused: ", err)
	}
	shutdownCancel()

	// Step 6: 等待所有goroutine结束
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
