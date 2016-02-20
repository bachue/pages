package log_driver

import (
	"log/syslog"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	conf "github.com/bachue/pages/config"
)

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

var levels = map[string]logrus.Level{
	"PANIC": logrus.PanicLevel,
	"FATAL": logrus.FatalLevel,
	"ERROR": logrus.ErrorLevel,
	"WARN":  logrus.WarnLevel,
	"INFO":  logrus.InfoLevel,
	"DEBUG": logrus.DebugLevel,
}

var syslogLevels = map[string]syslog.Priority{
	"DEBUG":   syslog.LOG_DEBUG,
	"INFO":    syslog.LOG_INFO,
	"NOTICE":  syslog.LOG_NOTICE,
	"WARNING": syslog.LOG_WARNING,
	"ERR":     syslog.LOG_ERR,
	"CRIT":    syslog.LOG_CRIT,
	"ALERT":   syslog.LOG_ALERT,
	"EMERG":   syslog.LOG_EMERG,
}

func New(config *conf.Log) (Logger, error) {
	logger := logrus.New()
	if strings.ToLower(config.Local) == "stderr" {
		logger.Out = os.Stderr
	} else if strings.ToLower(config.Local) == "stdout" {
		logger.Out = os.Stdout
	} else {
		file, err := os.Open(config.Local)
		if err != nil {
			return nil, err
		}
		logger.Out = file
	}
	logger.Level = levels[config.Level]

	if config.Syslog.Protocol != "" {
		level := syslogLevels[config.Syslog.Level]
		hook, err := logrus_syslog.NewSyslogHook(config.Syslog.Protocol, config.Syslog.Host, level, config.Syslog.Tag)
		if err != nil {
			return nil, err
		}
		logger.Hooks.Add(hook)
	}
	return logger, nil
}
