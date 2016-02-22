package log_driver

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/bachue/pages/config"
	"github.com/stretchr/testify/assert"
)

func TestNewLoggerWithoutSyslog(t *testing.T) {
	logConfig := &config.Log{Local: "STDOUT", Level: "WARN"}
	loggerInterface, err := New(logConfig)
	assert.Nil(t, err)
	logger, ok := loggerInterface.(*logrus.Logger)
	assert.True(t, ok)
	assert.Equal(t, logger.Out, os.Stdout)
	assert.Equal(t, logger.Level, logrus.WarnLevel)
	assert.Empty(t, logger.Hooks)
}
