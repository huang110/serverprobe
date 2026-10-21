package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// respondError 返回统一的错误响应
func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
	})
}

// parseIntDefault 解析整数查询参数，失败时返回默认值
func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// parseInt64Default 解析 int64 查询参数，失败时返回默认值
func parseInt64Default(value string, fallback int64) int64 {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

// paginationParams 解析并规范化分页参数
func paginationParams(c *gin.Context) (int, int) {
	page := parseIntDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}

	pageSize := parseIntDefault(c.Query("page_size"), 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// parseFloat 将缓存中的字符串转换为 float64，失败时返回 0
func parseFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// respondOK 返回统一成功响应
func respondOK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}
