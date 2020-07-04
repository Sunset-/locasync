package syncloc

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	logger "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"synclocation/model"
	"time"
)

import (
	_ "github.com/denisenkom/go-mssqldb"
)

const (
	_URL_PERSON_LOC = "/KjtxLocService.asmx/GetLocStatusInfo"
	_URL_SITES      = "/KjtxBaseDataService.asmx/GetWorkSiteInfo"
	_URL_AREAS      = "/KjtxBaseDataService.asmx/GetWorkArea"
	_URL_PERSON     = "/KjtxBaseDataService.asmx/GetEmployeeInfo"
)
const _CARD_TYPE_PERSON = "1" //人员
const _IS_IN_WELL = "1"       //井下
const _RES_SUCCESS = "1"      //成功

var HttpClient = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives:   false, //false 长链接 true 短连接
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        10 * 5, //client对与所有host最大空闲连接数总和
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,               //连接池对每个host的最大连接数量,当超出这个范围时，客户端会主动关闭到连接
		IdleConnTimeout:     60 * time.Second, //空闲连接在连接池中的超时时间
	},
	Timeout: 5 * time.Second,
}

type LocationSyncer struct {
	address  string
	interval time.Duration
	maxCols  int
	maxDepts int
	db       *sql.DB

	workAreas []*model.WorkArea
	workSites []*model.WorkSite
	depts     []string

	areaMap        map[string]*model.WorkArea
	siteCodeToArea map[string]string //基站id->区域名称 映射
	personDeptMap  map[string]string
}

func Start() {
	address := viper.GetString("address")
	interval := viper.GetInt("interval")
	if interval <= 0 {
		interval = 5
	}

	ls := &LocationSyncer{
		address:  address,
		interval: time.Duration(interval) * time.Second,
		maxCols:  viper.GetInt("cols"),
		maxDepts: viper.GetInt("depts"),
	}
	ls.siteCodeToArea = make(map[string]string, 0)
	ls.personDeptMap = make(map[string]string, 0)
	ls.areaMap = make(map[string]*model.WorkArea, 0)
	ls.depts = make([]string, 0)

	err := ls.initDB()
	if err != nil {
		logger.Warn(err)
		panic(err)
	}
	go ls.loop()
}

//初始化db
func (ls *LocationSyncer) initDB() error {
	db, err := sql.Open("mssql", viper.GetString("dsn"))
	if err != nil {
		return err
	}
	ls.db = db
	return nil
}

func (ls *LocationSyncer) loop() {
	for {
		time.Sleep(ls.interval)
		locParams, err := ls.loadPersonLocs()
		if err != nil {
			fmt.Println("请求人员定位异常：", err)
			continue
		}
		logger.Info(locParams.areaPersonCount)
		logger.Info(locParams.deptPersonCount)
		fmt.Println("区域个数:", len(locParams.areaPersonCount))
		fmt.Println("部门个数:", len(locParams.deptPersonCount))
		fmt.Println(locParams.areaPersonCount)
		fmt.Println(locParams.deptPersonCount)
		var t1, t2 int
		for _, t := range locParams.areaPersonCount {
			t1 += t
		}
		for _, t := range locParams.deptPersonCount {
			t2 += t
		}
		fmt.Println("区域总人数："+strconv.Itoa(t1), "部门总人数："+strconv.Itoa(t2))

		err = ls.saveLocToDB(locParams)
		if err != nil {
			logger.Warn("存储数据异常：", err)
		}
	}
}

type LocParams struct {
	areaPersonCount     map[string]int
	deptPersonCount     map[string]int
	locCount            int
	unInwellPersonCount int
	inwellPersonCount   int
	vehicleCount        int
	inwellVehicleCount  int

	leavePersonCount int

	area1Count int
	area2Count int
	area3Count int
	area4Count int
}

//请求第三方数据
func (ls *LocationSyncer) loadPersonLocs() (locParams *LocParams, err error) {
	locParams = &LocParams{
		areaPersonCount:     make(map[string]int, 0),
		deptPersonCount:     make(map[string]int, 0),
		locCount:            0,
		unInwellPersonCount: 0,
		inwellPersonCount:   0,
		vehicleCount:        0,
		inwellVehicleCount:  0,
		leavePersonCount:    0,

		area1Count: 0,
		area2Count: 0,
		area3Count: 0,
		area4Count: 0,
	}

	//请求人员信息
	locs := make([]*model.PersonLoc, 0)
	err = request(ls.address+_URL_PERSON_LOC, http.MethodPost, "application/x-www-form-urlencoded", "CardNum=&CardType=", &locs)
	if err != nil {
		return nil, err
	}
	if len(locs) == 0 {
		return locParams, nil
	}
	var unloadArea bool
	var unloadDept bool
	//var personUpLoc, personDownLoc, vehicleCount,inwellVehicleCount int

	for _, loc := range locs {
		if _, ok := ls.siteCodeToArea[loc.DevNum]; !ok {
			unloadArea = true
		}
		if loc.CardType == _CARD_TYPE_PERSON && loc.Isinwell == _IS_IN_WELL {
			if _, ok := ls.personDeptMap[loc.Cardnum]; !ok {
				unloadDept = true
			}
		}
	}
	if unloadArea {
		ls.loadWorkSites()
	}
	if unloadDept {
		ls.loadDepts()
	}
	//汇总
	for _, area := range ls.workAreas {
		locParams.areaPersonCount[area.Name] = 0
	}
	for _, dept := range ls.depts {
		locParams.deptPersonCount[dept] = 0
	}

	activeDuration := time.Duration(viper.GetInt("activeTime")) * time.Second
	leaveDuration := time.Duration(viper.GetInt("leaveTime")) * time.Second
	for _, loc := range locs {

		//离网人数
		if int(time.Now().Sub(Convert2Date(buquantime(loc.DevTime), DateFormatEN))) > int(leaveDuration) {
			if loc.CardType == _CARD_TYPE_PERSON {
				locParams.leavePersonCount++
			}
		}
		//移除时间异常的定位
		if int(time.Now().Sub(Convert2Date(buquantime(loc.DevTime), DateFormatEN))) > int(activeDuration) {
			fmt.Println("定位时间超出有效期：", loc.Cardnum, loc.DevTime)
			continue
		}

		if loc.CardType == _CARD_TYPE_PERSON && loc.Isinwell == _IS_IN_WELL {
			locParams.inwellPersonCount++

			if area, ok := ls.siteCodeToArea[loc.DevNum]; ok {
				locParams.areaPersonCount[area] += 1
				a, aok := ls.areaMap[area]
				if aok {
					switch a.TypeId {
					case "1":
						locParams.area1Count++
					case "2":
						locParams.area2Count++
					case "3":
						locParams.area3Count++
					case "4":
						locParams.area4Count++
					}
				}
			} else {
				fmt.Println("未找到基站：", loc.DevNum)
				logger.Warn("未找到基站：", loc.DevNum)
			}
			if dept, ok := ls.personDeptMap[loc.Cardnum]; ok {
				locParams.deptPersonCount[dept] += 1
			} else {
				fmt.Println("未找到组织机构：", loc.Cardnum)
				logger.Warn("未找到组织机构：", loc.Cardnum)
			}
		} else if loc.CardType != _CARD_TYPE_PERSON {
			locParams.vehicleCount++
			if loc.Isinwell == _IS_IN_WELL {
				locParams.inwellVehicleCount++
			}
		} else if loc.Isinwell != _IS_IN_WELL {
			locParams.unInwellPersonCount++
		}
	}
	fmt.Println("获取到定位信息总数：", len(locs))
	fmt.Println("车辆定位数：", locParams.vehicleCount)
	fmt.Println("井下车辆数：", locParams.inwellVehicleCount)
	fmt.Println("井上人员数：", locParams.unInwellPersonCount)
	fmt.Println("井下人员数：", locParams.inwellPersonCount)
	fmt.Println("离网人数：", locParams.leavePersonCount)

	locParams.locCount = len(locs)

	return locParams, nil
}

//获取基站信息
func (ls *LocationSyncer) loadWorkSites() {
	workAreas := make([]*model.WorkArea, 0)
	err := request(ls.address+_URL_AREAS, http.MethodPost, "application/x-www-form-urlencoded", "", &workAreas)
	if err != nil {
		logger.Warn("请求区域信息异常：", err)
		return
	}
	fmt.Println("获取到区域信息个数：", len(workAreas))
	sort.Sort(AreaSlice(workAreas))
	ls.workAreas = workAreas
	for _, a := range workAreas {
		ls.areaMap[a.Name] = a
	}
	workSites := make([]*model.WorkSite, 0)
	err = request(ls.address+_URL_SITES, http.MethodPost, "application/x-www-form-urlencoded", "number=&workAreaName=", &workSites)
	if err != nil {
		logger.Warn("请求基站信息异常：", err)
		return
	}
	ls.workSites = workSites
	fmt.Println("获取到基站信息个数：", len(workSites))
	for _, ws := range workSites {
		ls.siteCodeToArea[ws.Number] = ws.WorkAreaName
	}
}

//加载部门映射信息
func (ls *LocationSyncer) loadDepts() {
	persons := make([]*model.Person, 0)
	err := request(ls.address+_URL_PERSON, http.MethodPost, "application/x-www-form-urlencoded", "CardNum=&departmentName=&typeOfWorkName=&officePosition=", &persons)
	if err != nil {
		logger.Warn("请求人员信息异常：", err)
		return
	}
	fmt.Println("获取人员信息：", len(persons))
	depts := make([]string, 0)
	deptMap := make(map[string]bool, 0)
	for _, p := range persons {
		ls.personDeptMap[p.CardNumber] = p.Department
		if _, ok := deptMap[p.Department]; !ok {
			deptMap[p.Department] = true
			depts = append(depts, p.Department)
		}
	}
	sort.Sort(sort.StringSlice(depts))
	ls.depts = depts
}

type AreaSlice []*model.WorkArea

func (s AreaSlice) Len() int { return len(s) }

func (s AreaSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s AreaSlice) Less(i, j int) bool {
	i1, err1 := strconv.Atoi(s[i].Id)
	i2, err2 := strconv.Atoi(s[j].Id)
	if err1 == nil && err2 == nil {
		return i1 < i2
	}
	return s[i].Id < s[j].Id
}

//保存人员定位信息到数据库
func (ls *LocationSyncer) saveLocToDB(locParams *LocParams) error {
	//删除数据
	_, err := ls.db.Exec("DELETE FROM tb_inwell")
	if err != nil {
		logger.Warn("删除数据失败", err)
		return err
	}
	areas := ls.workAreas
	if ls.maxCols < len(ls.workAreas) {
		areas = ls.workAreas[:ls.maxCols]
	}
	depts := ls.depts
	if ls.maxDepts < len(ls.depts) {
		depts = ls.depts[:ls.maxDepts]
	}
	//拼接插入sql
	var header bytes.Buffer
	var row1 bytes.Buffer
	var row2 bytes.Buffer
	var row3 bytes.Buffer
	now := Time2StrF(time.Now(), "2006-01-02 15:04:05")
	header.WriteString("(UPDATE_TIME_,TOTAL_AREA_,TOTAL_SITE_,TOTAL_LOC_,TOTAL_VEHICLE_,INWELL_VEHICLE_,LEAVE_,AREA_1_,AREA_2_,AREA_3_,AREA_4_")
	row1.WriteString("('" + now + "','" + strconv.Itoa(len(ls.workAreas)) + "','" + strconv.Itoa(len(ls.workSites)) + "','" + strconv.Itoa(locParams.inwellPersonCount) + "','" + strconv.Itoa(locParams.inwellPersonCount) + "','" + strconv.Itoa(locParams.vehicleCount) + "','" + strconv.Itoa(locParams.inwellVehicleCount) + "','" + strconv.Itoa(locParams.area1Count) + "','" + strconv.Itoa(locParams.area2Count) + "','" + strconv.Itoa(locParams.area3Count) + "','" + strconv.Itoa(locParams.area4Count) + "'")
	row2.WriteString("('" + now + "','','',''")
	row3.WriteString("('" + now + "','区域总数','基站总数','井下人员数','车辆总数','井下车辆数','离网人数','区域类型1人数','区域类型2人数','区域类型3人数','区域类型4人数'")
	for i, area := range areas {
		header.WriteString(",COL_" + strconv.Itoa(i+1) + "_")
		row1.WriteString(",'" + strconv.Itoa(locParams.areaPersonCount[area.Name]) + "'")
		row2.WriteString(",'" + area.Id + "'")
		row3.WriteString(",'" + area.Name + "'")
	}
	for i, dept := range depts {
		header.WriteString(",DEP_" + strconv.Itoa(i+1) + "_")
		row1.WriteString(",'" + strconv.Itoa(locParams.deptPersonCount[dept]) + "'")
		row2.WriteString(",''")
		row3.WriteString(",'" + dept + "'")
	}
	header.WriteString(")")
	row1.WriteString(")")
	row2.WriteString(")")
	row3.WriteString(")")
	sql := "INSERT INTO tb_inwell " + header.String() + " VALUES " + row1.String() + "," + row2.String() + "," + row3.String()
	logger.Info("执行SQL：", sql)
	_, err = ls.db.Exec(sql)
	if err != nil {
		return err
	}
	return nil
}

type XmlWrap struct {
	XMLName xml.Name
	Data    []byte `xml:",innerxml"`
}
type Response struct {
	Code    string              `json:"Code"`
	Message string              `json:"Message"`
	Result  jsoniter.RawMessage `json:"result"`
}

//http请求
func request(url, method, contentType string, body string, resPointer interface{}) error {
	var bodyBytes []byte
	var resBytes []byte
	bodyBytes = []byte(body)
	logger.Debug("http-request:", url)
	if logger.GetLevel() == logger.DebugLevel {
		logger.Debug("http-request-params:", body)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	res, err := HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			logger.Warn("关闭res失败", err)
		}
	}()
	resBytes, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New(string(resBytes))
	}
	logger.Debug("http-response:", string(resBytes))
	xmlWrap := &XmlWrap{}
	err = xml.Unmarshal([]byte(strings.ReplaceAll(string(resBytes), "\n", "")), xmlWrap)
	if err != nil {
		return err
	}
	resWrap := &Response{}
	err = jsoniter.Unmarshal(xmlWrap.Data, resWrap)
	if err != nil {
		return err
	}
	if resWrap.Code != _RES_SUCCESS {
		return errors.New("请求失败：" + resWrap.Message)
	}
	if resPointer != nil {
		return jsoniter.Unmarshal(resWrap.Result, resPointer)
	}
	return nil
}

func buquantime(timeStr string) string {
	dates := strings.Split(timeStr, " ")
	if len(dates) != 2 {
		return timeStr
	}
	ymd := strings.Split(dates[0], "/")
	if len(ymd) != 3 {
		return timeStr
	}
	if len(ymd[1]) == 1 {
		ymd[1] = "0" + ymd[1]
	}
	if len(ymd[2]) == 1 {
		ymd[2] = "0" + ymd[2]
	}
	return strings.Join(ymd, "/") + " " + dates[1]
}
