package logic

import (
	"context"

	"ip_geo/internal/svc"
	"ip_geo/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetIpGeoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetIpGeoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetIpGeoLogic {
	return &GetIpGeoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetIpGeoLogic) GetIpGeo(req *types.GetIpGeoRequest) (resp *types.GetIpGeoResponse, err error) {
	l.Infof("GetIpGeo, req: %+v", *req)

	info, err := l.svcCtx.IpGeoHelper.QueryGeo(req.IpAddr)
	if err != nil {
		l.Errorf("query ip database failed, err: %v", err)
		return nil, err
	}

	resp = &types.GetIpGeoResponse{
		DBVersion:     info.DBVersion,
		ContinentCode: info.Continent,
		Country:       info.Country,
		CountryCode:   info.CountryCode,
		Region:        info.Region,
		City:          info.City,
		District:      info.District,
		AreaCode:      info.AreaCode,
		Isp:           info.Isp,
		ISPDomain:     info.IspDomain,
		ZipCode:       info.ZipCode,
		Latitude:      info.Latitude,
		Longitude:     info.Longitude,
		Timezone:      info.Timezone,
	}

	return resp, nil
}
