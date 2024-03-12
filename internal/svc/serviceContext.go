package svc

import (
	"fmt"
	"ip_geo/internal/config"
	"ip_geo/internal/middleware"
	"ip_geo/internal/model"
	"sync/atomic"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type ServiceContext struct {
	CfgPtr                *atomic.Pointer[config.Config]
	IpRateLimitMiddleware rest.Middleware
	RedisClient           *redis.Redis
	IpGeoHelper           model.IpGeoHelper
	GeoHelperReady        chan bool // 标识Helper是否ready
}

func NewServiceContext(cfgPtr *atomic.Pointer[config.Config]) *ServiceContext {
	var err error

	redisClient := redis.MustNewRedis(cfgPtr.Load().RedisConf)

	svcCtx := &ServiceContext{
		CfgPtr:                cfgPtr,
		RedisClient:           redisClient,
		IpRateLimitMiddleware: middleware.NewIpRateLimitMiddleware(cfgPtr, redisClient).Handle,
		GeoHelperReady:        make(chan bool),
	}

	helper, err := model.NewIpCloudDataHelper(cfgPtr)
	if err != nil {
		panic(fmt.Errorf("new ip cloud data helper failed: %v", err))
	}
	svcCtx.IpGeoHelper = helper

	// 初始化查询助手
	go func() {
		initErr := helper.Init()
		if initErr != nil {
			logx.Errorf("init ip geo helper failed: %v", initErr)
			panic(initErr)
		}
		close(svcCtx.GeoHelperReady)
	}()

	return svcCtx
}
