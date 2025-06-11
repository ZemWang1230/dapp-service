package main

import (
	"net/http"

	"timelocker-backend/internal/api/auth"
	"timelocker-backend/internal/config"
	"timelocker-backend/internal/repository/user"
	authService "timelocker-backend/internal/service/auth"
	"timelocker-backend/pkg/database"
	"timelocker-backend/pkg/logger"
	"timelocker-backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	logger.Init(logger.DefaultConfig())
	logger.Info("Starting Logger Success!")

	// 1. 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config: ", err)
	}

	// 2. 连接数据库
	db, err := database.NewPostgresConnection(&cfg.Database)
	if err != nil {
		logger.Error("Failed to connect to database: ", err)
	}

	// 3. 自动迁移数据库
	if err := database.AutoMigrate(db); err != nil {
		logger.Error("Failed to migrate database: ", err)
	}

	// 4. 创建索引
	if err := database.CreateIndexes(db); err != nil {
		logger.Error("Failed to create indexes: ", err)
	}

	// 5. 初始化仓库层
	userRepo := user.NewRepository(db)

	// 6. 初始化JWT管理器
	jwtManager := utils.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.AccessExpiry,
		cfg.JWT.RefreshExpiry,
	)

	// 7. 初始化服务层
	authSvc := authService.NewService(userRepo, jwtManager)

	// 8. 初始化处理器
	authHandler := auth.NewHandler(authSvc)

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

	// 12. 注册路由
	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
	}

	// 13. 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "timelocker-backend",
		})
	})

	// 14. 启动服务器
	addr := ":" + cfg.Server.Port
	logger.Info("Starting server on ", addr)

	if err := router.Run(addr); err != nil {
		logger.Error("Failed to start server: ", err)
	}
}
