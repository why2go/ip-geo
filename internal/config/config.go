package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	RedisConf      redis.RedisConf
	DataSyncConfig *DataSyncConfig
	RateLimit      *RateLimit
	AccessKey      string
	AccessSecret   string
}

// 离线数据同步配置
type DataSyncConfig struct {
	DownloadUrl    string // 离线数据下载地址
	SyncCron       string // 离线数据同步周期
	ForTest        bool   // 是否用于测试，用于测试时，不走cron表达式，改为每个一段时间更新一次
	RereshInterval string
}

type RateLimit struct {
	GlobalLimit int
	LimitPerIp  int
}
