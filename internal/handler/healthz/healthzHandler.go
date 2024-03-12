package healthz

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"ip_geo/internal/logic/healthz"
	"ip_geo/internal/svc"
)

func HealthzHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := healthz.NewHealthzLogic(r.Context(), svcCtx)
		err := l.Healthz()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.Ok(w)
		}
	}
}
