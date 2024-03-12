package model

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"ip_geo/internal/config"
	"ip_geo/internal/utils"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/go-co-op/gocron/v2"
	"github.com/zeromicro/go-zero/core/logx"
)

var (
	_ IpGeoHelper = (*IpCloudDataHelper)(nil)
)

type IpCloudDataHelper struct {
	syncer    gocron.Scheduler
	curDbPtr  atomic.Pointer[ipDataCloudDb]
	newDbPtr  atomic.Pointer[ipDataCloudDb]
	cfgPtr    *atomic.Pointer[config.Config]
	version   atomic.Value
	refreshMu sync.Mutex
}

func NewIpCloudDataHelper(cfgPtr *atomic.Pointer[config.Config]) (*IpCloudDataHelper, error) {
	var err error
	helper := &IpCloudDataHelper{}
	syncer, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return nil, err
	}
	cfg := cfgPtr.Load()
	if cfg.DataSyncConfig.ForTest {
		duration, err := time.ParseDuration(cfg.DataSyncConfig.RereshInterval)
		if err != nil {
			return nil, err
		}
		if duration < 5*time.Second {
			return nil, fmt.Errorf("refresh interval less than 5 seconds: %s", cfg.DataSyncConfig.RereshInterval)
		}
		j, err := syncer.NewJob(gocron.DurationJob(duration), gocron.NewTask(helper.refreshDb))
		if err != nil {
			return nil, err
		}
		logx.Infof("refresh db job id for test: %s", j.ID())
	} else {
		j, err := syncer.NewJob(gocron.CronJob(cfg.DataSyncConfig.SyncCron, false),
			gocron.NewTask(helper.refreshDb))
		if err != nil {
			return nil, err
		}
		logx.Infof("refresh db job id: %s", j.ID())
	}
	helper.cfgPtr = cfgPtr
	helper.syncer = syncer
	helper.curDbPtr.Store(&ipDataCloudDb{data: new(bytes.Buffer)})
	helper.newDbPtr.Store(&ipDataCloudDb{data: new(bytes.Buffer)})

	return helper, nil
}

func (helper *IpCloudDataHelper) QueryGeo(ipAddr string) (resp *GeoInfo, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			resp, err = nil, fmt.Errorf("panic: %v", panicErr)
		}
	}()
	str, err := helper.curDbPtr.Load().getRecordStr(ipAddr)
	if err != nil {
		return nil, err
	}

	infos := strings.Split(str, "|")
	if len(infos) < 16 {
		return nil, fmt.Errorf("wrong number of record fields: %d, at least 16, but got: %s", len(infos), str)
	}
	ips := &ipDataCloudGeoInfo{
		Continent:   infos[0],  //洲
		Country:     infos[1],  //国家/地区
		Province:    infos[2],  //省份
		City:        infos[3],  //城市
		Line:        infos[4],  //线路
		Isp:         infos[5],  //运营商
		AreaCode:    infos[6],  //区域代码
		CountryCode: infos[7],  //国家/地区英文简写
		Longitude:   infos[8],  //经度
		Latitude:    infos[9],  //纬度
		ZipCode:     infos[10], //邮编
		Asn:         infos[11], //asn
		Domain:      infos[12], //运营商域名
		Idc:         infos[13], //idc
		Station:     infos[14], //基站
		TimeZone:    infos[15], //时区
	}

	resp = &GeoInfo{
		DBVersion:   helper.version.Load().(string),
		Continent:   utils.GetContinentCodeByName(ips.Continent),
		Country:     ips.Country,
		CountryCode: ips.CountryCode,
		Region:      ips.Province,
		City:        ips.City,
		AreaCode:    ips.AreaCode,
		Isp:         ips.Isp,
		IspDomain:   ips.Domain,
		ZipCode:     ips.ZipCode,
		Latitude:    ips.Latitude,
		Longitude:   ips.Longitude,
		Timezone:    ips.TimeZone,
	}

	return resp, nil
}

// 初始化db
func (helper *IpCloudDataHelper) Init() error {
	err := helper.doRefreshDb() // 先同步一次
	if err != nil {
		return err
	}

	helper.syncer.Start() // 启动定时刷新
	return nil
}

// 清理
func (helper *IpCloudDataHelper) Clean() error {
	err := helper.syncer.Shutdown()
	if err != nil {
		logx.Errorf("shutdown refresh job failed: %v", err)
	}
	return nil
}

func (helper *IpCloudDataHelper) refreshDb() {
	err := helper.doRefreshDb()
	if err != nil {
		logx.Errorf("error refreshing ip cloud data db: %v", err)
	}
}

func (helper *IpCloudDataHelper) doRefreshDb() (err error) {
	helper.refreshMu.Lock()
	defer helper.refreshMu.Unlock()
	logx.Infof("begin refreshing ip cloud data db")
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("%v", panicErr)
		}
	}()

	var filepath string

	for i := 0; i < 2; i++ { // 下载错误，则重试一次
		filepath, err = helper.downloadOfflineDb(helper.cfgPtr.Load().DataSyncConfig.DownloadUrl)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	logx.Infof("finish downloading ip data cloud db, path: %s", filepath)

	uncompFilepath, err := helper.uncompressDbFile(filepath)
	if err != nil {
		return err
	}
	logx.Infof("finish uncompressing ip data cloud db, path: %s", uncompFilepath)

	db, err := helper.loadFile(uncompFilepath)
	if err != nil {
		return err
	}
	logx.Infof("finish load ip data cloud db file")

	// 做一次查询，来简单验证数据库是否正确
	testIp := "10.0.0.1"
	str, err := db.getRecordStr(testIp)
	if err != nil {
		return err
	}
	if n := len(strings.Split(str, "|")); n < 16 {
		return fmt.Errorf("wrong number of record fields: %d, at least 16, but got: %s", n, str)
	}
	logx.Infof("finish testing ip data cloud db, test ip: %s", testIp)

	oldDbPtr := helper.curDbPtr.Load()

	version := time.Now().Format(time.DateOnly)
	helper.version.Store(version)
	helper.curDbPtr.Store(db)
	helper.newDbPtr.Store(oldDbPtr)

	// 删除下载文件
	err = os.Remove(filepath)
	if err != nil {
		logx.Errorf("remove downloaded file failed, path: %s", filepath)
	}

	// 删除解压文件
	err = os.Remove(uncompFilepath)
	if err != nil {
		logx.Errorf("remove uncompressed file failed, path: %s", uncompFilepath)
	}

	logx.Infof("done refresh ip cloud data db, version: %v", helper.version.Load().(string))

	return nil
}

func (helper *IpCloudDataHelper) downloadOfflineDb(fileUri string) (filepath string, err error) {
	ctx, cf := context.WithTimeout(context.Background(), 30*time.Minute) // 设置最长下载时长为30min
	defer cf()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileUri, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download file failed, fileUri: %v, expected status code 200, but got %d", fileUri, resp.StatusCode)
	}

	// 如果返回json格式，则报错了
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		b := new(bytes.Buffer)
		io.Copy(b, resp.Body)
		resp.Body.Close()
		return "", fmt.Errorf("download file failed, fileUri: %v, resp body: %s", fileUri, b.String())
	}

	// 创建一个临时文件
	f, err := os.CreateTemp("", "*")
	if err != nil {
		return "", err
	}
	// 将下载文件拷贝到临时文件中
	_, err = io.Copy(f, resp.Body)
	resp.Body.Close()
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	filepath = f.Name()
	return filepath, nil
}

// 解压zip格式的原始文件
func (helper *IpCloudDataHelper) uncompressDbFile(compressedFilepath string) (uncompressedFilepath string, err error) {
	// Open a zip archive for reading.
	r, err := zip.OpenReader(compressedFilepath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	if len(r.File) == 0 {
		return "", errors.New("no file in zip")
	}

	// 应该只有一个文件
	f := r.File[0]
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	newFile, err := os.CreateTemp("", "*")
	if err != nil {
		return "", err
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, rc)
	if err != nil {
		return "", err
	}

	return newFile.Name(), nil
}

// 需要保证文件的完整性，任何解析都可能出错
func (helper *IpCloudDataHelper) loadFile(file string) (*ipDataCloudDb, error) {
	unpackInt4byte := func(a, b, c, d byte) uint32 {
		return (uint32(a) & 0xFF) | ((uint32(b) << 8) & 0xFF00) | ((uint32(c) << 16) & 0xFF0000) | ((uint32(d) << 24) & 0xFF000000)
	}

	var err error

	p := helper.newDbPtr.Load()
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p.data.Reset()
	_, err = io.Copy(p.data, f)
	if err != nil {
		return nil, err
	}
	data := p.data.Bytes()

	for k := 0; k < 256; k++ {
		i := k*8 + 4
		p.prefStart[k] = unpackInt4byte(data[i], data[i+1], data[i+2], data[i+3])
		p.prefEnd[k] = unpackInt4byte(data[i+4], data[i+5], data[i+6], data[i+7])
	}

	RecordSize := int(unpackInt4byte(data[0], data[1], data[2], data[3]))

	p.endArr = p.endArr[:0]
	p.addrArr = p.addrArr[:0]
	for i := 0; i < RecordSize; i++ {
		j := 2052 + (i * 9)
		endipnum := unpackInt4byte(data[j], data[1+j], data[2+j], data[3+j])
		offset := unpackInt4byte(data[4+j], data[5+j], data[6+j], data[7+j])
		length := uint32(data[8+j])
		p.endArr = append(p.endArr, endipnum)
		buf := data[offset:int(offset+length)]
		p.addrArr = append(p.addrArr, unsafe.String(unsafe.SliceData(buf), len(buf)))
	}

	return p, err
}

type ipDataCloudGeoInfo struct {
	Continent   string `json:"continent"`    //洲
	Country     string `json:"country"`      //国家/地区
	Province    string `json:"province"`     //省份
	City        string `json:"city"`         //城市
	Line        string `json:"line"`         //线路
	Isp         string `json:"isp"`          //运营商
	AreaCode    string `json:"area_code"`    //区域代码
	CountryCode string `json:"country_code"` //国家/地区英文简写
	Longitude   string `json:"longitude"`    //经度
	Latitude    string `json:"latitude"`     //纬度
	ZipCode     string `json:"zip_code"`     //邮编
	Asn         string `json:"asn"`          //asn
	Domain      string `json:"domain"`       //运营商域名
	Idc         string `json:"idc"`          //idc
	Station     string `json:"station"`      //基站
	TimeZone    string `json:"time_zone"`    //时区
}

type ipDataCloudDb struct {
	prefStart [256]uint32
	prefEnd   [256]uint32
	endArr    []uint32
	addrArr   []string
	data      *bytes.Buffer
}

func (p *ipDataCloudDb) getRecordStr(ip string) (string, error) {
	ips := strings.Split(ip, ".")
	x, err := strconv.Atoi(ips[0])
	if err != nil {
		return "", err
	}
	prefix := uint32(x)
	intIP, err := p.ipToInt(ip)
	if err != nil {
		return "", err
	}

	low := p.prefStart[prefix]
	high := p.prefEnd[prefix]

	var cur uint32
	if low == high {
		cur = low
	} else {
		cur = p.search(low, high, intIP)
	}
	if cur == 100000000 {
		return "", errors.New("not found")
	} else {
		return p.addrArr[cur], nil
	}
}

func (p *ipDataCloudDb) search(low uint32, high uint32, k uint32) uint32 {
	var M uint32 = 0
	for low <= high {
		mid := (low + high) / 2
		endipNum := p.endArr[mid]
		if endipNum >= k {
			M = mid
			if mid == 0 {
				break
			}
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	return M
}

func (p *ipDataCloudDb) ipToInt(ipstr string) (uint32, error) {
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return 0, errors.New("invalid ip")
	}
	ip = ip.To4()
	return binary.BigEndian.Uint32(ip), nil
}
