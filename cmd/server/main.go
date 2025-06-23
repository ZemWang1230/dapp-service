package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"timelocker-backend/docs"
	"timelocker-backend/internal/api/auth"
	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/token"
	"timelocker-backend/internal/repository/user"
	authService "timelocker-backend/internal/service/auth"
	priceService "timelocker-backend/internal/service/price"
	"timelocker-backend/pkg/database"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// http://localhost:8080/swagger/index.html
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
// @Summary 健康检查
// @Description 检查服务健康状态
// @Tags 系统
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "timelocker-backend",
	})
}

func main() {
	logger.Init(logger.DefaultConfig())
	logger.Info("Starting Logger Success!")

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

	// 4. 自动迁移数据库
	if err := database.AutoMigrate(db); err != nil {
		logger.Error("Failed to migrate database: ", err)
		os.Exit(1)
	}

	// 5. 创建索引
	if err := database.CreateIndexes(db); err != nil {
		logger.Error("Failed to create indexes: ", err)
		os.Exit(1)
	}

	// 6. 初始化仓库层
	userRepo := user.NewRepository(db)
	tokenRepo := token.NewRepository(db)

	// 7. 初始化价格服务
	priceSvc := priceService.NewService(&cfg.Price, tokenRepo, redisClient)

	// 8. 启动价格服务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := priceSvc.Start(ctx); err != nil {
		logger.Error("Failed to start price service: ", err)
		os.Exit(1)
	}

	// 9. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 10. 初始化服务层
	authSvc := authService.NewService(userRepo, jwtManager)

	// 11. 初始化处理器
	authHandler := auth.NewHandler(authSvc)

	// 12. 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 13. 创建路由器
	router := gin.Default()

	// 14. 添加CORS中间件
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

	// 15. 注册路由
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
	}

	// Swagger API文档端点
	docs.SwaggerInfo.Host = "localhost:" + cfg.Server.Port
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 16. 健康检查端点
	router.GET("/health", healthCheck)

	// 17. 设置优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("Received shutdown signal, stopping services...")

		// 停止价格服务
		if err := priceSvc.Stop(); err != nil {
			logger.Error("Failed to stop price service: ", err)
		}

		cancel()
		os.Exit(0)
	}()

	// 18. 启动服务器
	addr := ":" + cfg.Server.Port
	logger.Info("Starting server on ", addr)

	if err := router.Run(addr); err != nil {
		logger.Error("Failed to start server: ", err)
		os.Exit(1)
	}
}
