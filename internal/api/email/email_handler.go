package email

import (
	"net/http"
	"strconv"
	"timelocker-backend/internal/middleware"
	"timelocker-backend/internal/service/auth"
	"timelocker-backend/internal/service/email"
	"timelocker-backend/internal/types"
	"timelocker-backend/pkg/logger"

	"github.com/gin-gonic/gin"
)

// EmailHandler 邮箱API处理器
type EmailHandler struct {
	emailService email.EmailService
	authService  auth.Service
}

// NewEmailHandler 创建邮箱处理器实例
func NewEmailHandler(emailService email.EmailService, authService auth.Service) *EmailHandler {
	return &EmailHandler{
		emailService: emailService,
		authService:  authService,
	}
}

// RegisterRoutes 注册邮箱相关路由
func (h *EmailHandler) RegisterRoutes(router *gin.RouterGroup) {
	// 邮箱API组 - 需要认证
	emailGroup := router.Group("/emails", middleware.AuthMiddleware(h.authService))
	{
		// 邮箱管理
		// 添加邮箱
		// POST /api/v1/emails
		// http://localhost:8080/api/v1/emails
		emailGroup.POST("", h.AddEmail)
		// 获取邮箱列表
		// GET /api/v1/emails
		// http://localhost:8080/api/v1/emails
		emailGroup.GET("", h.GetEmails)
		// 更新邮箱备注
		// PUT /api/v1/emails/{id}/remark
		// http://localhost:8080/api/v1/emails/1/remark
		emailGroup.PUT("/:id/remark", h.UpdateEmailRemark)
		// 删除邮箱
		// DELETE /api/v1/emails/{id}
		// http://localhost:8080/api/v1/emails/1
		emailGroup.DELETE("/:id", h.DeleteEmail)

		// 邮箱验证
		// 发送验证码
		// POST /api/v1/emails/send-verification
		// http://localhost:8080/api/v1/emails/send-verification
		emailGroup.POST("/send-verification", h.SendVerificationCode)
		// 验证邮箱
		// POST /api/v1/emails/verify
		// http://localhost:8080/api/v1/emails/verify
		emailGroup.POST("/verify", h.VerifyEmail)
	}
}

// ===== 邮箱管理相关API =====

// AddEmail 添加邮箱
// @Summary 添加邮箱
// @Description 为当前用户添加新的邮箱地址，当前API返回邮箱ID，需要配合邮箱验证API使用
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.AddEmailRequest true "添加邮箱请求"
// @Success 200 {object} types.APIResponse{data=types.AddEmailResponse}
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 409 {object} types.APIResponse{error=types.APIError} "邮箱已存在"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "参数校验失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails [post]
func (h *EmailHandler) AddEmail(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{
			Success: false,
			Error:   &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"},
		})
		return
	}

	var req types.AddEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Success: false,
			Error:   &types.APIError{Code: "INVALID_REQUEST", Message: "Invalid request body", Details: err.Error()},
		})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	result, err := h.emailService.AddUserEmail(c.Request.Context(), userIDInt, req.Email, req.Remark)
	if err != nil {
		logger.Error("Failed to add email", err, "userID", userIDInt, "email", req.Email)
		if err.Error() == "email already added by user" {
			c.JSON(http.StatusConflict, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_EXISTS", Message: "Email already added"}})
			return
		}
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to add email", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    types.AddEmailResponse{ID: result.ID, Message: "Email added successfully"},
	})
}

// GetEmails 获取用户邮箱列表
// @Summary 获取用户邮箱列表
// @Description 获取当前用户的所有邮箱地址
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码，默认为1" default(1)
// @Param page_size query int false "每页大小，默认为10" default(10)
// @Success 200 {object} types.APIResponse{data=types.EmailListResponse}
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails [get]
func (h *EmailHandler) GetEmails(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{Success: false, Error: &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"}})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	// 解析分页参数
	page := 1
	pageSize := 10
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	result, err := h.emailService.GetUserEmails(c.Request.Context(), userIDInt, page, pageSize)
	if err != nil {
		logger.Error("Failed to get user emails", err, "userID", userIDInt)
		c.JSON(http.StatusOK, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to get emails", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    result,
	})
}

// UpdateEmailRemark 更新邮箱备注
// @Summary 更新邮箱备注
// @Description 更新指定邮箱的备注信息
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户邮箱ID"
// @Param request body types.UpdateEmailRemarkRequest true "更新备注请求"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权限操作该邮箱"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮箱不存在"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "参数校验失败"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails/{id}/remark [put]
func (h *EmailHandler) UpdateEmailRemark(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{Success: false, Error: &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"}})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	// 获取邮箱ID
	userEmailIDStr := c.Param("id")
	userEmailID, err := strconv.ParseInt(userEmailIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_PARAMS", Message: "Invalid email ID"}})
		return
	}

	var req types.UpdateEmailRemarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_REQUEST", Message: "Invalid request body", Details: err.Error()}})
		return
	}

	err = h.emailService.UpdateEmailRemark(c.Request.Context(), userEmailID, userIDInt, req.Remark)
	if err != nil {
		logger.Error("Failed to update email remark", err, "userID", userIDInt, "userEmailID", userEmailID)
		if err.Error() == "user email not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_NOT_FOUND", Message: "Email not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to update email remark", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Email remark updated successfully"},
	})
}

// DeleteEmail 删除邮箱
// @Summary 删除邮箱
// @Description 删除指定的邮箱地址
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户邮箱ID"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 403 {object} types.APIResponse{error=types.APIError} "无权限操作该邮箱"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮箱不存在"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails/{id} [delete]
func (h *EmailHandler) DeleteEmail(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{Success: false, Error: &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"}})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	// 获取邮箱ID
	userEmailIDStr := c.Param("id")
	userEmailID, err := strconv.ParseInt(userEmailIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_PARAMS", Message: "Invalid email ID"}})
		return
	}

	err = h.emailService.DeleteUserEmail(c.Request.Context(), userEmailID, userIDInt)
	if err != nil {
		logger.Error("Failed to delete email", err, "userID", userIDInt, "userEmailID", userEmailID)
		if err.Error() == "user email not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_NOT_FOUND", Message: "Email not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to delete email", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Email deleted successfully"},
	})
}

// ===== 邮箱验证相关API =====

// SendVerificationCode 发送验证码
// @Summary 发送验证码
// @Description 向指定邮箱发送验证码，如果AddEmail返回成功，则使用返回的ID发送验证码即可；若AddEmail返回邮箱存在，则直接调用此API发送验证码（即重发验证码）
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.SendVerificationCodeRequest true "发送验证码请求"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮箱不存在"
// @Failure 429 {object} types.APIResponse{error=types.APIError} "发送过于频繁"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails/send-verification [post]
func (h *EmailHandler) SendVerificationCode(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{Success: false, Error: &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"}})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	var req types.SendVerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_REQUEST", Message: "Invalid request body", Details: err.Error()}})
		return
	}

	err := h.emailService.SendVerificationCode(c.Request.Context(), req.UserEmailID, userIDInt)
	if err != nil {
		logger.Error("Failed to send verification code", err, "userID", userIDInt, "userEmailID", req.UserEmailID)
		switch err.Error() {
		case "user email not found":
			c.JSON(http.StatusNotFound, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_NOT_FOUND", Message: "Email not found", Details: err.Error()}})
			return
		case "email already verified":
			c.JSON(http.StatusConflict, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_ALREADY_VERIFIED", Message: "Email already verified", Details: err.Error()}})
			return
		case "verification code sent recently, please wait":
			c.JSON(http.StatusTooManyRequests, types.APIResponse{Success: false, Error: &types.APIError{Code: "TOO_MANY_REQUESTS", Message: "Verification code sent recently, please wait", Details: err.Error()}})
			return
		default:
			c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to send verification code", Details: err.Error()}})
			return
		}
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Verification code sent successfully"},
	})
}

// VerifyEmail 验证邮箱
// @Summary 验证邮箱
// @Description 使用验证码验证邮箱地址
// @Tags Email
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body types.VerifyEmailRequest true "验证邮箱请求"
// @Success 200 {object} types.APIResponse
// @Failure 400 {object} types.APIResponse{error=types.APIError} "请求参数错误"
// @Failure 401 {object} types.APIResponse{error=types.APIError} "未授权"
// @Failure 404 {object} types.APIResponse{error=types.APIError} "邮箱不存在"
// @Failure 422 {object} types.APIResponse{error=types.APIError} "验证码无效或已过期"
// @Failure 500 {object} types.APIResponse{error=types.APIError} "服务器内部错误"
// @Router /api/v1/emails/verify [post]
func (h *EmailHandler) VerifyEmail(c *gin.Context) {
	// 获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, types.APIResponse{Success: false, Error: &types.APIError{Code: "UNAUTHORIZED", Message: "User not authenticated"}})
		return
	}

	userIDInt, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Invalid user ID format"}})
		return
	}

	var req types.VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("Invalid request body", err)
		c.JSON(http.StatusBadRequest, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_REQUEST", Message: "Invalid request body", Details: err.Error()}})
		return
	}

	err := h.emailService.VerifyEmail(c.Request.Context(), req.UserEmailID, userIDInt, req.Code)
	if err != nil {
		logger.Error("Failed to verify email", err, "userID", userIDInt, "userEmailID", req.UserEmailID)
		if err.Error() == "user email not found" {
			c.JSON(http.StatusNotFound, types.APIResponse{Success: false, Error: &types.APIError{Code: "EMAIL_NOT_FOUND", Message: "Email not found"}})
			return
		}
		if err.Error() == "invalid or expired verification code" || err.Error() == "failed to verify code: invalid or expired verification code" {
			c.JSON(http.StatusUnprocessableEntity, types.APIResponse{Success: false, Error: &types.APIError{Code: "INVALID_OR_EXPIRED_CODE", Message: "Invalid or expired verification code"}})
			return
		}
		c.JSON(http.StatusInternalServerError, types.APIResponse{Success: false, Error: &types.APIError{Code: "INTERNAL_ERROR", Message: "Failed to verify email", Details: err.Error()}})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Success: true,
		Data:    gin.H{"message": "Email verified successfully"},
	})
}
