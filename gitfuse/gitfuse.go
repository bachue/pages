package gitfuse

import (
	"io/ioutil"
	"os"

	conf "github.com/bachue/pages/config"
	"github.com/bachue/pages/log_driver"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type GitFs struct {
	pathfs.FileSystem
	GitRepoDir string
	GitFsDir   string
	server     *fuse.Server
}

func New(config *conf.Fuse, logger log_driver.Logger) (*GitFs, error) {
	gitfsDir, err := gitfsDir(logger)
	if err != nil {
		return nil, err
	}

	defaultfs := pathfs.NewDefaultFileSystem()
	gitfs := &GitFs{FileSystem: pathfs.NewReadonlyFileSystem(defaultfs), GitRepoDir: config.GitRepoDir, GitFsDir: gitfsDir}
	fs := pathfs.NewPathNodeFs(gitfs, nil)
	server, _, err := nodefs.MountRoot(gitfsDir, fs.Root(), nil)
	if err != nil {
		logger.Errorf("Failed to mount GitFS on %s due to %s", gitfsDir, err)
		return nil, err
	}
	gitfs.server = server
	server.SetDebug(config.Debug)

	return gitfs, nil
}

func (gitfs *GitFs) Start() {
	defer os.RemoveAll(gitfs.GitFsDir)
	gitfs.server.Serve()
}

func gitfsDir(logger log_driver.Logger) (string, error) {
	dir, err := ioutil.TempDir("", "gitfs")
	if err != nil {
		logger.Errorf("Failed to create a temporary dir due to %s", err)
		return "", err
	}
	return dir, nil
}
