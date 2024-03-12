package healthz

import (
	"context"
	"fmt"

	"ip_geo/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthzLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHealthzLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthzLogic {
	return &HealthzLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HealthzLogic) Healthz() error {
	select {
	case <-l.svcCtx.GeoHelperReady:
		return nil
	default:
		return fmt.Errorf("ip geo helper not ready")
	}
}
