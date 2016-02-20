package gitfuse

import (
	"io/ioutil"
	"testing"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"github.com/stretchr/testify/assert"
)

func TestGitFsSetup(t *testing.T) {
	_, cleaner := setupGitFsTest(t)
	defer cleaner()
}

func setupGitFsTest(t *testing.T) (*GitFs, func()) {
	dir, err := ioutil.TempDir("", "gitfs-test")
	assert.Nil(t, err)

	fsConfig := &config.Fuse{GitRepoDir: dir, Debug: false}
	logConfig := &config.Log{Local: "STDERR", Level: "WARN"}
	logger, err := log_driver.New(logConfig)
	gitfs, err := New(fsConfig, logger)
	assert.Nil(t, err)
	go gitfs.Start()
	return gitfs, func() {
		err := gitfs.server.Unmount()
		assert.Nil(t, err)
	}
}
