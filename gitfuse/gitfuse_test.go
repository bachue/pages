package gitfuse

import (
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
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

	stat, ok := files[0].Sys().(*syscall.Stat_t)
	assert.True(t, ok)
	assert.EqualValues(t, stat.Nlink, 5)

	assert.EqualValues(t, files[1].Name(), "flightjs")
	assert.True(t, files[1].Mode().IsDir())

	stat, ok = files[1].Sys().(*syscall.Stat_t)
	assert.True(t, ok)
	assert.EqualValues(t, stat.Nlink, 5)

	files, err = ioutil.ReadDir(gitfs.GitFsDir + "/pry")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 2)
	assert.EqualValues(t, files[0].Name(), "pry")
	assert.True(t, files[0].Mode().IsDir())

	stat, ok = files[0].Sys().(*syscall.Stat_t)
	assert.True(t, ok)
	assert.EqualValues(t, stat.Nlink, 7)

	assert.EqualValues(t, files[1].Name(), "ruby-pry")
	assert.True(t, files[1].Mode().IsDir())

	stat, ok = files[1].Sys().(*syscall.Stat_t)
	assert.True(t, ok)
	assert.EqualValues(t, stat.Nlink, 5)

	files, err = ioutil.ReadDir(gitfs.GitFsDir + "/remnux")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 1)
	assert.EqualValues(t, files[0].Name(), "remnux")
	assert.True(t, files[0].Mode().IsDir())

	stat, ok = files[0].Sys().(*syscall.Stat_t)
	assert.True(t, ok)
	assert.EqualValues(t, stat.Nlink, 5)
}

func TestGitFsReadThirdLayer(t *testing.T) {
	gitfs, cleaner := setupGitFsTest(t)
	defer cleaner()

	files, err := ioutil.ReadDir(gitfs.GitFsDir + "/flightjs/example-app")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 12)

	assert.EqualValues(t, files[0].Name(), ".gitattributes")
	assert.False(t, files[0].Mode().IsDir())
	assert.EqualValues(t, files[1].Name(), ".gitignore")
	assert.False(t, files[1].Mode().IsDir())
	assert.EqualValues(t, files[2].Name(), ".travis.yml")
	assert.False(t, files[2].Mode().IsDir())
	assert.EqualValues(t, files[3].Name(), "LICENSE.md")
	assert.False(t, files[3].Mode().IsDir())
	assert.EqualValues(t, files[4].Name(), "README.md")
	assert.False(t, files[4].Mode().IsDir())
	assert.EqualValues(t, files[5].Name(), "app")
	assert.True(t, files[5].Mode().IsDir())
	assert.EqualValues(t, files[6].Name(), "bower_components")
	assert.True(t, files[6].Mode().IsDir())
	assert.EqualValues(t, files[7].Name(), "index.html")
	assert.False(t, files[7].Mode().IsDir())
	assert.EqualValues(t, files[8].Name(), "karma.conf.js")
	assert.False(t, files[8].Mode().IsDir())
	assert.EqualValues(t, files[9].Name(), "package.json")
	assert.False(t, files[9].Mode().IsDir())
	assert.EqualValues(t, files[10].Name(), "requireMain.js")
	assert.False(t, files[10].Mode().IsDir())
	assert.EqualValues(t, files[11].Name(), "test")
	assert.True(t, files[11].Mode().IsDir())
}

func TestGitFsReadFourthLayer(t *testing.T) {
	gitfs, cleaner := setupGitFsTest(t)
	defer cleaner()

	files, err := ioutil.ReadDir(gitfs.GitFsDir + "/flightjs/flightjs/css")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 2)

	assert.EqualValues(t, files[0].Name(), "main.css")
	assert.False(t, files[0].Mode().IsDir())
	assert.EqualValues(t, files[1].Name(), "normalize.min.css")
	assert.False(t, files[1].Mode().IsDir())

	files, err = ioutil.ReadDir(gitfs.GitFsDir + "/pry/ruby-pry/bin")
	assert.Nil(t, err)

	assert.EqualValues(t, len(files), 1)

	assert.EqualValues(t, files[0].Name(), "pry")
	assert.False(t, files[0].Mode().IsDir())
	assert.EqualValues(t, files[0].Mode().Perm(), 0555)
}

func TestGitFsXAttr(t *testing.T) {
	gitfs, cleaner := setupGitFsTest(t)
	defer cleaner()

	xattrs := make([]byte, 0)
	sz, err := unix.Listxattr(gitfs.GitFsDir+"/pry/ruby-pry/bin/pry", xattrs)
	assert.Nil(t, err)
	assert.EqualValues(t, sz, 0)
	assert.Len(t, xattrs, 0)

	xattr := make([]byte, 0)
	sz, err = unix.Getxattr(gitfs.GitFsDir+"/pry/ruby-pry/bin/pry", "a.b.c", xattr)
	assert.EqualValues(t, err, unix.ENODATA)
	assert.EqualValues(t, sz, -1)
	assert.Len(t, xattr, 0)
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
		err := gitfs.Unmount()
		assert.Nil(t, err)
		err = os.RemoveAll(dir)
		assert.Nil(t, err)
	}
}
