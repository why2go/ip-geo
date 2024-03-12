// Code generated by goctl. DO NOT EDIT.
package types

type GetIpGeoRequest struct {
	IpAddr string `form:"ip_addr"`
}

type GetIpGeoResponse struct {
	DBVersion     string `json:"db_version"`     // 数据库版本
	ContinentCode string `json:"continent_code"` // 大洲代码
	Country       string `json:"country"`        // 国家/地区
	CountryCode   string `json:"country_code"`   // 国家代码
	Region        string `json:"region"`         // 省、州
	City          string `json:"city"`           // 城市
	District      string `json:"district"`       // 区县
	AreaCode      string `json:"area_code"`      // 区域代码
	Isp           string `json:"isp"`            // 运营商
	ISPDomain     string `json:"isp_domain"`     // 运营商域名
	ZipCode       string `json:"zip_code"`       // 邮编
	Latitude      string `json:"latitude"`       // 纬度
	Longitude     string `json:"longitude"`      // 经度
	Timezone      string `json:"timezone"`       // 时区
}