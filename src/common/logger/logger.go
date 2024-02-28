package logger

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	Trace   *log.Logger //任何东西
	Info    *log.Logger //重要信息
	Warning *log.Logger //警告
	Error   *log.Logger //错误
)

func ColorString(prefix string, color string) string {
	// ANSI转义码：\033[颜色代码m
	// 颜色代码：31-红色，32-绿色，33-黄色，34-蓝色，35-紫色，36-青色，37-白色
	var colorCode string
	switch color {
	case "red": // 红色
		colorCode = "\033[31m"
	case "green": // 绿色
		colorCode = "\033[32m"
	case "yellow": // 黄色
		colorCode = "\033[33m"
	case "blue": // 蓝色
		colorCode = "\033[34m"
	case "purple": // 紫色
		colorCode = "\033[35m"
	default: // 默认颜色
		colorCode = ""
	}
	// 在输出前添加颜色代码
	prefix = fmt.Sprintf("%s%s\033[0m ", colorCode, prefix)
	return prefix
}

// 设置日志级别
func setLogLevel(level logrus.Level) {
	//建一个新的Logger对象
	logger := logrus.New()
	// 设置日志级别为DEBUG
	logger.SetLevel(level)
}

// 日志记录器函数
func logger() {
	file, err := os.OpenFile("errors.logger", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Unable to open logger file:", err)
	}
	// 在输出前添加颜色代码
	prefix := ColorString("[TRACE:]", "purple")
	Trace = log.New(ioutil.Discard,
		prefix,
		log.Ldate|log.Lmicroseconds|log.Llongfile)

	prefix = ColorString("[INFO:]", "green")
	Info = log.New(os.Stdout,
		prefix,
		log.Ldate|log.Lmicroseconds|log.Llongfile)
	prefix = ColorString("[WARNING:]", "yellow")
	Warning = log.New(os.Stdout,
		prefix,
		log.Ldate|log.Lmicroseconds|log.Llongfile)
	prefix = ColorString("[ERROR:]", "red")
	Error = log.New(io.MultiWriter(file, os.Stderr),
		prefix,
		log.Ldate|log.Lmicroseconds|log.Llongfile)
}

func init() {
	//设置日志级别为debug
	setLogLevel(logrus.DebugLevel)
	logger()
}
