package goldsky

// import (
// 	"timelocker-backend/internal/service/goldsky"
// )

// // SyncHandler Goldsky 同步处理器
// type SyncHandler struct {
// 	goldskyService *goldsky.GoldskyService
// }

// // NewSyncHandler 创建同步处理器
// func NewSyncHandler(goldskyService *goldsky.GoldskyService) *SyncHandler {
// 	return &SyncHandler{
// 		goldskyService: goldskyService,
// 	}
// }

// // RegisterRoutes 注册路由
// func (h *SyncHandler) RegisterRoutes(router *gin.RouterGroup) {
// 	// http://localhost:8080/api/v1/goldsky/sync
// 	router.POST("/goldsky/sync", h.SyncFlows)
// }

// SyncFlows 手动同步所有flows
// @Summary 手动同步所有Goldsky flows
// @Description 手动触发所有链的flows同步，与定时任务执行相同的逻辑
// @Tags Goldsky
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=types.SyncFlowsResponse}
// @Failure 500 {object} types.APIResponse{error=types.APIError} "同步过程中发生错误"
// @Router /api/v1/goldsky/sync [post]
// func (h *SyncHandler) SyncFlows(c *gin.Context) {
// 	startTime := time.Now()
// 	logger.Info("Starting manual sync of Goldsky flows")

// 	// 执行同步
// 	err := h.goldskyService.SyncAllFlowsNow(c.Request.Context())
// 	if err != nil {
// 		logger.Error("Failed to sync Goldsky flows", err)
// 		c.JSON(http.StatusInternalServerError, types.APIResponse{
// 			Success: false,
// 			Error: &types.APIError{
// 				Code:    "SYNC_FAILED",
// 				Message: "Failed to sync Goldsky flows",
// 				Details: err.Error(),
// 			},
// 		})
// 		return
// 	}

// 	duration := time.Since(startTime)
// 	logger.Info("Successfully completed manual sync of Goldsky flows", "duration", duration)

// 	c.JSON(http.StatusOK, types.APIResponse{
// 		Success: true,
// 		Data: types.SyncFlowsResponse{
// 			Message:  "Flows synchronized successfully",
// 			Duration: duration.String(),
// 		},
// 	})
// }
