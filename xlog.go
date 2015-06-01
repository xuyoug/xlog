package xlog

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Curr version 1.2
// author xuyoug
// mail:xuyoug@yeah.net
// 2015-05-26
//

// 调用方式
// 依次传入：文件名、buf大小、log切换大小格式的字符串、和按时间切换的类型
// l, err := xlog.NewXlog("log/log-{date}-{index}.log", 10000, "10M", xlog.TruncByday)
// 	defer l.Close()
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// l.Log("hello", 123, "asd")
// l.WriteString("test","test")
// l.Witef("this is %d day",i)

// 不定义日志级别（这个东西很坑，大家需求都不一样，请需要的朋友在外层自定义实现）
// 统一在log每一行前加上了时间，精确到秒

// version 1.1
// 2015-04-27
// 再次优化程序结构
// 增加注释
// ticker的优化
// 设定超时后不自动切换log
// 修复设置不按大小切换后，文件名中仍然具有计数的bug
// 不按时间切换log时的bug
// 不按大小切换log的实现
// 修复一些其它BUG
// version 1.0
// 修复bug
// 可以应用于生产环境
// 2015-4-26
// version 0.1
// fix cpu占用的bug
// 结构的优化

const (
	TruncByDay  = "DAY"
	TruncByHour = "HOUR"
	TruncByMin  = "MIN"
	NoTrunc     = "NO"
)

const (
	DefaultLogName string = "Log.log"
	DefaultLogSize int64  = 50 * 1000 * 1000
)

//getFileSize获取文件大小
func getFileSize(file *os.File) (size int64, err error) {
	var tmpfileinfo os.FileInfo
	tmpfileinfo, err = file.Stat()
	if err != nil {
		return 0, err
	}
	return tmpfileinfo.Size(), err
}

//getFmtSize根据设置的大小格式，判断切换大小
func getFmtSize(s string) (size int64, err error) {
	st := strings.ToUpper(s)
	if st == "" {
		return DefaultLogSize, nil
	}
	var ts string
	var ti int64
	_, err = fmt.Sscanf(st, "%d%s", &ti, &ts)
	if err != nil {
		if err == io.EOF {
			_, err = fmt.Sscanf(st, "%d", &ti)
		}
		if err != nil {
			panic("Error in format switch size")
			return 0, err
		}
	}
	if ti == 0 {
		return 0, nil
	}
	switch ts {
	case "K", "KB":
		return ti * 1000, nil
	case "M", "MB":
		return ti * 1000 * 1000, nil
	case "G", "GB":
		return ti * 1000 * 1000 * 1000, nil
	case "B", "":
		return ti, nil
	default:
		return DefaultLogSize, nil
	}
	return 0, nil
}

//getBaseLog进行当前应该使用的log名的获取
func (x *Xlog) getBaseLog() (d string) {
	truncnameold := x.truncname
	if x.trunc == "" || x.logname == "" {
		x.logname = DefaultLogName
	}
	switch strings.ToUpper(x.trunc) {
	case TruncByDay:
		x.truncname = time.Now().Format("20060102")
	case TruncByHour:
		x.truncname = time.Now().Format("2006010215")
	case TruncByMin:
		x.truncname = time.Now().Format("200601021504")
	case NoTrunc:
		x.truncname = time.Now().Format("20060102") //不按时间切换的话则按启动日期记录
	default:
		x.truncname = ""
	}
	if truncnameold != x.truncname { //按时间切换之后，日志计数重新开始
		x.curIndex = 0
	}
	d = strings.Replace(x.logname, "{DATE}", x.truncname, -1)
	d = strings.Replace(d, "{date}", x.truncname, -1)

	if x.swcsize == 0 { //不按大小切换的话，直接不理睬
		d = strings.Replace(d, "{index}", "", -1)
		d = strings.Replace(d, "{INDEX}", "", -1)
		return d
	}
	index := fmt.Sprintf("%03d", x.curIndex)
	d = strings.Replace(d, "{INDEX}", index, -1)
	d = strings.Replace(d, "{index}", index, -1)
	return d
}

//switchlog进行log切换
func (x *Xlog) switchlog() (is bool) {
	//判断是否需要根据大小切换
	//对于自动到达相同log组最后一个的情况，如果需要判断大小仍然进行大小判断，不判断的话就继续写
	var err error
	tmpi1 := x.curIndex
	if x.swcsize != 0 {
		x.checkSize()
	}
	tmpi2 := x.curIndex
	oldLogname := x.curLogname
	//判断是否需要根据时间切换
	var newLogname string
	if x.trunc != NoTrunc {
		newLogname = x.getBaseLog()
	} else {
		newLogname = oldLogname
	}
	//如果都不需要则不切换log
	if tmpi1 == tmpi2 || oldLogname == newLogname {
		return false
	}

	// open new file fist
	var tmp_curFile *os.File
	tmp_curFile, err = os.OpenFile(newLogname, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
	var oldcurFile *os.File
	if err != nil {
		panic("Error in switch log (open new file)")
	} else {
		oldcurFile = x.curFile
		// if there is no err then use new file
		x.curFile = tmp_curFile
		x.curLogname = newLogname
	}
	// close old file then
	err = oldcurFile.Close()
	if err != nil {
		panic("Error in switch log (close old file)")
	}
	return true
}

//checkSize检查文件大小是否达到切换标准，达到的话标记一下。不切换的话不标记
func (x *Xlog) checkSize() {
	size, err := getFileSize(x.curFile)
	if x.swcsize != 0 && (size >= x.swcsize || err != nil) {
		x.curIndex += 1
	}
}

//isExistFile判断文件是否存在
func (x *Xlog) isExistFile() bool {
	_, err := os.Stat(x.getBaseLog())
	return err == nil || os.IsExist(err)
}

//gotoLastFile用于启动时如果已经存在相同log组的话到达最后一个log
func (x *Xlog) gotoLastFile() {
	for {
		if x.isExistFile() {
			x.curIndex += 1
			continue
		}
		x.curIndex -= 1
		if x.curIndex < 0 {
			x.curIndex = 0
		}
		break
	}
}

//Xlog对象定义
type Xlog struct {
	logname     string
	curLogname  string
	curFile     *os.File
	curIndex    int
	c           chan string
	trunc       string
	switchsize  string
	swcsize     int64
	logCurindex int
	ticker      *time.Ticker
	truncname   string
}

//NewXlog初始化函数 依次传入：文件名、buf大小、log切换大小格式的字符串、和按时间切换的类型
func NewXlog(s string, bufsize int, swcsizes string, tr string) (*Xlog, error) {
	x := new(Xlog)
	var err error
	x.logname = s
	x.logCurindex = 0
	x.trunc = tr
	x.switchsize = swcsizes
	x.swcsize, _ = getFmtSize(swcsizes)
	x.curIndex = 0
	x.c = make(chan string, bufsize)
	if x.swcsize != 0 {
		x.gotoLastFile()
	}
	x.curLogname = x.getBaseLog()
	if x.swcsize != 0 || x.trunc != NoTrunc {
		x.ticker = time.NewTicker(time.Millisecond * 200) //每200ms进行一次切换检测
	}
	x.curFile, err = os.OpenFile(x.curLogname, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0777)
	go x.pushlog()
	return x, err
}

//Close关闭log对象：关闭缓存chan，关闭ticker 注意：要当所有的内容写完之后才会进行关闭
func (x *Xlog) Close() (err error) {
	for {
		if x.BufDep() == 0 {
			close(x.c)
			err = x.curFile.Close()
			x.ticker.Stop()
			return err
		} else {
			time.Sleep(time.Nanosecond * 10000)
		}
	}
}

//GetBufDep对外提供，获取缓存chan的深度
func (x *Xlog) BufDep() int {
	return len(x.c)
}

//WriteString对外提供的3个写入方法之一
func (x *Xlog) Println(s ...interface{}) {
	tmps := fmt.Sprintln(s...)
	tmps = time.Now().Format("2006-01-02 15:04:05 ") + tmps
	x.c <- tmps
}

//Writef对外提供的3个写入方法之一
func (x *Xlog) Printf(fmts string, i ...interface{}) {
	tmps := fmt.Sprintf(fmts, i...)
	tmps = time.Now().Format("2006-01-02 15:04:05 ") + tmps
	x.c <- tmps
}

//pushlog作为独立gorotiune执行  负责获取缓存并写入文件
//负责ticker的调度
//实现无操作时的休眠
func (x *Xlog) pushlog() {
	var sleeptimes int
	var issleep bool = false
	for {
		select {
		case s, ok := <-x.c:
			if issleep { //从休眠中唤醒需要先切换log、重新设置ticker
				x.switchlog()
				x.ticker = time.NewTicker(time.Millisecond * 200)
			}
			if ok {
				x.curFile.WriteString(s)
				sleeptimes = 0 //写入log后重置休眠计数器
			}
			//对于ticker的处理，加入了休眠计数
		case <-x.ticker.C:
			if sleeptimes > 1000 { //1000次tick都无写入则进入休眠
				x.ticker.Stop()
				issleep = true
			}
			x.switchlog()
			sleeptimes++
		}
	}
}

//God luck！
