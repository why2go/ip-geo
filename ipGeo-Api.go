package main

import (
	"flag"
	"fmt"
	"sync/atomic"

	"ip_geo/internal/config"
	"ip_geo/internal/handler"
	"ip_geo/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var (
	configFile = flag.String("f", "etc/ipGeo-Api.yaml", "the config file")
)

func main() {
	flag.Parse()

	cfgPtr := &atomic.Pointer[config.Config]{}
	c := &config.Config{}
	if err := conf.FillDefault(c); err != nil {
		panic(fmt.Errorf("failed to fill default config when get config from file, err: %v", err))
	}
	conf.MustLoad(*configFile, c, conf.UseEnv())
	cfgPtr.Store(c)

	server := rest.MustNewServer(cfgPtr.Load().RestConf, rest.WithCors())
	defer server.Stop()

	ctx := svc.NewServiceContext(cfgPtr)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", cfgPtr.Load().Host, cfgPtr.Load().Port)
	server.Start()
}
