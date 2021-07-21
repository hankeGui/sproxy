package log

import (
	"github.com/gxlog/gxlog"
	"github.com/gxlog/gxlog/formatter/text"
	"github.com/gxlog/gxlog/iface"
)

var logger = gxlog.Logger()

func init() {
	gxlog.Formatter().EnableColoring()
	gxlog.Formatter().SetHeader("{{time:time.ms}} [{{level}}] {{msg}}\n")
	gxlog.Formatter().SetColor(iface.Debug, text.BrightBlue)
}

func Error(v ...interface{}) {
	logger.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

func Warn(v ...interface{}) {
	logger.Warn(v...)
}

func Warnf(format string, v ...interface{}) {
	logger.Warnf(format, v...)
}

func Info(v ...interface{}) {
	logger.Info(v...)
}

func Infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

func Debug(v ...interface{}) {
	logger.Debug(v...)
}

func Debugf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

func Trace(v ...interface{}) {
	logger.Trace(v...)
}

func Tracef(format string, v ...interface{}) {
	logger.Tracef(format, v...)
}

func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func SetLevel(level string) {
	var l iface.Level

	switch level {
	case "error", "ERROR":
		l = iface.Error
	case "warn", "WARN":
		l = iface.Warn
	case "debug", "DEBUG":
		l = iface.Debug
	case "trace", "TRACE":
		l = iface.Trace
	default:
		l = iface.Info
	}

	logger.SetLevel(l)
}
