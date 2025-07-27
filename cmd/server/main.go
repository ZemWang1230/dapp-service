package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"timelocker-backend/docs"
	abiHandler "timelocker-backend/internal/api/abi"
	assetHandler "timelocker-backend/internal/api/asset"
	authHandler "timelocker-backend/internal/api/auth"
	chainHandler "timelocker-backend/internal/api/chain"
	emailNotificationHandler "timelocker-backend/internal/api/email_notification"
	sponsorHandler "timelocker-backend/internal/api/sponsor"
	timelockHandler "timelocker-backend/internal/api/timelock"
	transactionHandler "timelocker-backend/internal/api/transaction"
	"timelocker-backend/internal/config"
	abiRepo "timelocker-backend/internal/repository/abi"
	assetRepo "timelocker-backend/internal/repository/asset"
	chainRepo "timelocker-backend/internal/repository/chain"
	emailNotificationRepo "timelocker-backend/internal/repository/email_notification"
	sponsorRepo "timelocker-backend/internal/repository/sponsor"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	transactionRepo "timelocker-backend/internal/repository/transaction"
	userRepo "timelocker-backend/internal/repository/user"
	abiService "timelocker-backend/internal/service/abi"
	assetService "timelocker-backend/internal/service/asset"
	authService "timelocker-backend/internal/service/auth"
	chainService "timelocker-backend/internal/service/chain"
	emailNotificationService "timelocker-backend/internal/service/email_notification"
	sponsorService "timelocker-backend/internal/service/sponsor"
	timelockService "timelocker-backend/internal/service/timelock"
	transactionService "timelocker-backend/internal/service/transaction"
	"timelocker-backend/pkg/blockchain"
	"timelocker-backend/pkg/database"
	"timelocker-backend/pkg/email"
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

// healthCheck 健康检查端点
// @Summary 服务健康检查
// @Description 检查TimeLocker后端服务的健康状态，返回服务状态、服务名称和版本信息。此接口用于监控系统可用性。
// @Tags System
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "服务健康状态正常"
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "timelocker-backend",
		"version": "2.0.0",
	})
}

func main() {
	logger.Init(logger.DefaultConfig())
	logger.Info("Starting TimeLocker Backend v2.0.0")

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
	redisClient, err := database.NewRedisConnection(&cfg.Redis)
	if err != nil {
		logger.Error("Failed to connect to Redis: ", err)
		os.Exit(1)
	}

	// 4. 初始化仓库层
	userRepository := userRepo.NewRepository(db)
	chainRepository := chainRepo.NewRepository(db)
	assetRepository := assetRepo.NewRepository(db)
	abiRepository := abiRepo.NewRepository(db)
	sponsorRepository := sponsorRepo.NewRepository(db)
	timelockRepository := timelockRepo.NewRepository(db)
	transactionRepository := transactionRepo.NewRepository(db)
	emailNotificationRepository := emailNotificationRepo.NewRepository(db)

	// 5. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 6. 初始化邮件服务
	emailSvc := email.NewService(&cfg.Email)

	// 7. 初始化服务层
	authSvc := authService.NewService(userRepository, jwtManager)
	assetSvc := assetService.NewService(
		&cfg.Covalent,
		userRepository,
		chainRepository,
		assetRepository,
		redisClient,
	)
	abiSvc := abiService.NewService(abiRepository)
	chainSvc := chainService.NewService(chainRepository)
	sponsorSvc := sponsorService.NewService(sponsorRepository)
	timelockSvc := timelockService.NewService(timelockRepository)
	transactionSvc := transactionService.NewService(transactionRepository, timelockRepository)
	emailNotificationSvc := emailNotificationService.NewService(emailNotificationRepository, emailSvc, &cfg.Email)

	// 7. 初始化区块链监听器
	eventListener := blockchain.NewEventListener(cfg, transactionSvc, emailNotificationSvc, chainRepository, timelockRepository, db)
	go func() {
		if err := eventListener.Start(context.Background()); err != nil {
			logger.Error("Failed to start event listener: ", err)
		}
	}()

	// 8. 初始化处理器
	authHandler := authHandler.NewHandler(authSvc)
	assetHandler := assetHandler.NewHandler(assetSvc, authSvc)
	abiHandler := abiHandler.NewHandler(abiSvc, authSvc)
	chainHandler := chainHandler.NewHandler(chainSvc)
	sponsorHdl := sponsorHandler.NewHandler(sponsorSvc, authSvc)
	timelockHandler := timelockHandler.NewHandler(timelockSvc, authSvc)
	transactionHandler := transactionHandler.NewHandler(transactionSvc, authSvc)
	emailNotificationHdl := emailNotificationHandler.NewHandler(emailNotificationSvc, authSvc)

	// 9. 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 10. 创建路由器
	router := gin.Default()

	// 11. 添加CORS中间件
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

	// 12. 注册API路由
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
		assetHandler.RegisterRoutes(v1)
		abiHandler.RegisterRoutes(v1)
		chainHandler.RegisterRoutes(v1)
		sponsorHdl.RegisterRoutes(v1)
		timelockHandler.RegisterRoutes(v1)
		transactionHandler.RegisterRoutes(v1)
		emailNotificationHdl.RegisterRoutes(v1)
	}

	// 13. Swagger API文档端点
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	docs.SwaggerInfo.Title = "TimeLocker Backend API v2.0"
	docs.SwaggerInfo.Description = "TimeLocker Backend API with Transaction Management"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 14. 健康检查端点
	router.GET("/health", healthCheck)

	// 15. 系统状态端点
	router.GET("/api/v1/system/rpc-status", func(c *gin.Context) {
		// 从数据库获取启用RPC的链配置
		chains, err := chainRepository.GetRPCEnabledChains(c.Request.Context(), cfg.RPC.IncludeTestnets)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to get chain configurations: " + err.Error(),
			})
			return
		}

		status := make(map[string]interface{})

		for _, chainInfo := range chains {
			rpcURL, err := cfg.GetRPCURL(&chainInfo)

			chainStatus := map[string]interface{}{
				"chain_id":     chainInfo.ChainID,
				"display_name": chainInfo.DisplayName,
				"rpc_enabled":  chainInfo.RPCEnabled,
				"is_testnet":   chainInfo.IsTestnet,
			}

			if err != nil {
				chainStatus["status"] = "error"
				chainStatus["error"] = err.Error()
			} else {
				chainStatus["status"] = "configured"
				chainStatus["rpc_url"] = rpcURL
				chainStatus["has_api_key"] = !strings.Contains(rpcURL, "YOUR_API_KEY") &&
					!strings.Contains(rpcURL, "YOUR_ALCHEMY_API_KEY") &&
					!strings.Contains(rpcURL, "YOUR_INFURA_API_KEY")
			}

			status[chainInfo.ChainName] = chainStatus
		}

		response := map[string]interface{}{
			"provider": cfg.RPC.Provider,
			"api_keys": map[string]interface{}{
				"alchemy": map[string]interface{}{
					"configured": cfg.RPC.AlchemyAPIKey != "" && cfg.RPC.AlchemyAPIKey != "YOUR_ALCHEMY_API_KEY",
				},
				"infura": map[string]interface{}{
					"configured": cfg.RPC.InfuraAPIKey != "" && cfg.RPC.InfuraAPIKey != "YOUR_INFURA_API_KEY",
				},
			},
			"include_testnets": cfg.RPC.IncludeTestnets,
			"chains":           status,
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    response,
		})
	})

	// 16. 设置优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Received shutdown signal, stopping services...")

		// 停止区块链监听器
		eventListener.Stop()

		os.Exit(0)
	}()

	// 17. 启动服务器
	addr := ":" + cfg.Server.Port
	logger.Info("Starting server on ", "address", addr)
	logger.Info("Swagger documentation available at: http://localhost:" + cfg.Server.Port + "/swagger/index.html")

	if err := router.Run(addr); err != nil {
		logger.Error("Failed to start server: ", err)
		os.Exit(1)
	}
}
