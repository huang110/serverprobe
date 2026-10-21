package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
	limitergin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

// DefaultRequestsPerSecond 默认每秒请求数
const DefaultRequestsPerSecond = 100

// RateLimitMiddleware 基于内存令牌桶的限流中间件
func RateLimitMiddleware(requestsPerSecond int64) gin.HandlerFunc {
	if requestsPerSecond <= 0 {
		requestsPerSecond = DefaultRequestsPerSecond
	}

	rate := limiter.Rate{
		Period: time.Second,
		Limit:  requestsPerSecond,
	}
	store := memory.NewStore()
	instance := limiter.New(store, rate)

	return limitergin.NewMiddleware(instance)
}
