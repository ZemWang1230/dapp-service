package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"timelocker-backend/docs"
	assetHandler "timelocker-backend/internal/api/asset"
	authHandler "timelocker-backend/internal/api/auth"
	chainHandler "timelocker-backend/internal/api/chain"
	timelockHandler "timelocker-backend/internal/api/timelock"
	"timelocker-backend/internal/config"
	assetRepo "timelocker-backend/internal/repository/asset"
	chainRepo "timelocker-backend/internal/repository/chain"
	timelockRepo "timelocker-backend/internal/repository/timelock"
	userRepo "timelocker-backend/internal/repository/user"
	assetService "timelocker-backend/internal/service/asset"
	authService "timelocker-backend/internal/service/auth"
	chainService "timelocker-backend/internal/service/chain"
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
	userRepo := userRepo.NewRepository(db)
	chainRepo := chainRepo.NewRepository(db)
	assetRepo := assetRepo.NewRepository(db)
	timelockRepository := timelockRepo.NewRepository(db)

	// 5. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 6. 初始化服务层
	authSvc := authService.NewService(userRepo, jwtManager)
	assetSvc := assetService.NewService(
		&cfg.Covalent,
		userRepo,
		chainRepo,
		assetRepo,
		redisClient,
	)
	chainSvc := chainService.NewService(chainRepo)
	timelockSvc := timelockService.NewService(timelockRepository)

	// 7. 初始化处理器
	authHandler := authHandler.NewHandler(authSvc)
	assetHandler := assetHandler.NewHandler(assetSvc, authSvc)
	chainHandler := chainHandler.NewHandler(chainSvc)
	timelockHandler := timelockHandler.NewHandler(timelockSvc, authSvc)

	// 8. 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 9. 创建路由器
	router := gin.Default()

	// 10. 添加CORS中间件
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

	// 11. 注册路由
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
		assetHandler.RegisterRoutes(v1)
		chainHandler.RegisterRoutes(v1)
		timelockHandler.RegisterRoutes(v1)
	}

	// 12. Swagger API文档端点
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	docs.SwaggerInfo.Title = "TimeLocker Backend API v1.0"
	docs.SwaggerInfo.Description = "TimeLocker Backend API"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 13. 健康检查端点
	router.GET("/health", healthCheck)

	// 14. 设置优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Received shutdown signal, stopping services...")
		os.Exit(0)
	}()

	// 15. 启动服务器
	addr := ":" + cfg.Server.Port
	logger.Info("Starting server on ", addr)
	logger.Info("Swagger documentation available at: http://localhost:" + cfg.Server.Port + "/swagger/index.html")

	if err := router.Run(addr); err != nil {
		logger.Error("Failed to start server: ", err)
		os.Exit(1)
	}
}
