package syncloc

import (
	"strconv"
	"strings"
	"time"
)

const (
	defultTimeLayout     string = "2006-01-02 15:04:05"
	DateFormatDefault    string = "2006-01-02 15:04:05"
	DateFormatNoSpan     string = "20060102150405"
	DateFormatLongNoSpan string = "20060102150405000"
	DateFormatEN         string = "2006/01/02 15:04:05"
)

var CstZone = time.FixedZone("CST", 8*3600)

/**字符串->时间对象*/
func Str2Time(formatTimeStr string) time.Time {

	if !strings.Contains(formatTimeStr, ":") {
		if len(formatTimeStr) == len(DateFormatNoSpan) {
			return Convert2Date(formatTimeStr, DateFormatNoSpan)
		} else if len(formatTimeStr) > len(DateFormatNoSpan) {
			t := Convert2Date(formatTimeStr[:len(DateFormatNoSpan)], DateFormatNoSpan)
			subS, err := strconv.Atoi(formatTimeStr[len(DateFormatNoSpan):])
			if err == nil {
				nano := subS
				ws := 9 - len(formatTimeStr[len(DateFormatNoSpan):])
				for i := 0; i < ws; i++ {
					nano *= 10
				}
				t = t.Add(time.Duration(nano) * time.Nanosecond)
			}
			return t
		}
	}
	if strings.Contains(formatTimeStr, "/") {
		if len(formatTimeStr) > len(DateFormatEN) {
			formatTimeStr = subStrForMath(formatTimeStr)
		}
		return Convert2Date(formatTimeStr, DateFormatEN)
	}
	return Convert2Date(formatTimeStr, defultTimeLayout)

}

/**
去除字符串中中文
*/
func subStrForMath(str string) string {
	r := []rune(str)
	var cnstr string
	for i := 0; i < len(r); i++ {
		if r[i] >= 40869 || r[i] <= 19968 {
			cnstr = cnstr + string(r[i])

		}
		//fmt.Println("r[", i, "]=", r[i], "string=", string(r[i]))
	}
	return cnstr
}
func Convert2Date(formatTimeStr string, strDateFormat string) time.Time {
	theTime, _ := time.ParseInLocation(strDateFormat, formatTimeStr, CstZone) //使用模板在对应时区转化为time.time类型
	return theTime
}

/**字符串->时间戳*/
func Str2Stamp(formatTimeStr string) int64 {
	if !(len(formatTimeStr) > 0) {
		return 0
	}
	timeStruct := Str2Time(formatTimeStr)
	millisecond := timeStruct.UnixNano() / 1e6
	return millisecond
}

/**时间对象->字符串*/
func Time2Str() string {
	t := time.Now()
	temp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), CstZone)
	str := temp.Format(defultTimeLayout)
	return str
}

func Time2StrWithFormat(format string, timeLong int64) string {
	t := Stamp2Time(timeLong)
	temp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), CstZone)
	str := temp.Format(format)
	return str
}

/*时间对象->时间戳*/
func Time2Stamp() int64 {
	t := time.Now()
	millisecond := t.UnixNano() / 1e6
	return millisecond
}

/*时间戳->字符串*/
func Stamp2Str(stamp int64, timeLayout string) string {
	str := time.Unix(stamp/1000, 0).In(CstZone).Format(timeLayout)
	return str
}

/*时间戳->时间对象*/
func Stamp2Time(stamp int64) time.Time {
	stampStr := Stamp2Str(stamp, defultTimeLayout)
	timer := Str2Time(stampStr)
	return timer
}

/**时间对象->字符串*/
func Time2StrF(t time.Time, formatStr string) string {
	temp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), CstZone)
	str := temp.Format(formatStr)
	return str
}
