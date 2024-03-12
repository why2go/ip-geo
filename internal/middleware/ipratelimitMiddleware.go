package middleware

import (
	"fmt"
	"ip_geo/internal/config"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/zeromicro/go-zero/core/limit"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type IpRateLimitMiddleware struct {
	cfgPtr *atomic.Pointer[config.Config]
	redis  *redis.Redis
}

func NewIpRateLimitMiddleware(cfgPtr *atomic.Pointer[config.Config], redis *redis.Redis) *IpRateLimitMiddleware {
	return &IpRateLimitMiddleware{
		cfgPtr: cfgPtr,
		redis:  redis,
	}
}

func (m *IpRateLimitMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.cfgPtr.Load() != nil && m.cfgPtr.Load().RateLimit.LimitPerIp > 0 {
			ips := strings.Split(httpx.GetRemoteAddr(r), ",")
			if len(ips) > 0 {
				ipStr := strings.TrimSpace(ips[0])
				ip := net.ParseIP(ipStr)
				if ip.IsGlobalUnicast() && !ip.IsPrivate() { // 只限制公网单播地址
					limiter := limit.NewTokenLimiter(
						m.cfgPtr.Load().RateLimit.LimitPerIp,
						m.cfgPtr.Load().RateLimit.LimitPerIp,
						m.redis,
						m.getIpRateLimitKey(m.cfgPtr.Load(), ipStr),
					)
					if !limiter.Allow() {
						logx.Alert("limit exceeded, ip: " + ipStr)
						w.WriteHeader(http.StatusTooManyRequests)
						return
					}
				}
			}
		}
		// Passthrough to next handler if need
		next(w, r)
	}
}

func (m *IpRateLimitMiddleware) getIpRateLimitKey(c *config.Config, ipStr string) string {
	return fmt.Sprintf("%s:rate_limit:ip:%s", c.Name, ipStr)
}
