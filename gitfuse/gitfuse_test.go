package gitfuse

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"github.com/stretchr/testify/assert"
)

func TestGitFsReadFirstLayer(t *testing.T) {
	gitfs, cleaner := setupGitFsTest(t)
	defer cleaner()

	files, err := ioutil.ReadDir(gitfs.GitFsDir)
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 3)
	assert.EqualValues(t, files[0].Name(), "flightjs")
	assert.True(t, files[0].Mode().IsDir())
	assert.EqualValues(t, files[1].Name(), "pry")
	assert.True(t, files[1].Mode().IsDir())
	assert.EqualValues(t, files[2].Name(), "remnux")
	assert.True(t, files[2].Mode().IsDir())
}

func TestGitFsReadSecondLayer(t *testing.T) {
	gitfs, cleaner := setupGitFsTest(t)
	defer cleaner()

	files, err := ioutil.ReadDir(gitfs.GitFsDir + "/flightjs")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 2)
	assert.EqualValues(t, files[0].Name(), "example-app")
	assert.True(t, files[0].Mode().IsDir())
	assert.EqualValues(t, files[1].Name(), "flightjs")
	assert.True(t, files[1].Mode().IsDir())

	files, err = ioutil.ReadDir(gitfs.GitFsDir + "/pry")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 1)
	assert.EqualValues(t, files[0].Name(), "pry")
	assert.True(t, files[0].Mode().IsDir())

	files, err = ioutil.ReadDir(gitfs.GitFsDir + "/remnux")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 1)
	assert.EqualValues(t, files[0].Name(), "remnux")
	assert.True(t, files[0].Mode().IsDir())
}

func setupGitFsTest(t *testing.T) (*GitFs, func()) {
	dir, err := ioutil.TempDir("", "gitfs-test")
	assert.Nil(t, err)

	cmd := exec.Command("tar", "xvf", "pages.tar.gz", "-C", dir)
	err = cmd.Run()
	assert.Nil(t, err)

	fsConfig := &config.Fuse{GitRepoDir: dir, Debug: false}
	logConfig := &config.Log{Local: "STDERR", Level: "WARN"}
	logger, err := log_driver.New(logConfig)
	assert.Nil(t, err)
	gitfs, err := New(fsConfig, logger)
	assert.Nil(t, err)
	go gitfs.Start()
	gitfs.WaitStart()
	return gitfs, func() {
		err := gitfs.server.Unmount()
		assert.Nil(t, err)
		err = os.RemoveAll(dir)
		assert.Nil(t, err)
	}
}
