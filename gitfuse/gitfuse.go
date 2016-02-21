package gitfuse

import (
	"io/ioutil"
	"os"
	"strings"

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
	logger     log_driver.Logger
}

func New(config *conf.Fuse, logger log_driver.Logger) (*GitFs, error) {
	gitfsDir, err := gitfsDir(logger)
	if err != nil {
		return nil, err
	}

	defaultfs := pathfs.NewDefaultFileSystem()
	gitfs := &GitFs{FileSystem: pathfs.NewReadonlyFileSystem(defaultfs), GitRepoDir: config.GitRepoDir, GitFsDir: gitfsDir, logger: logger}
	fs := pathfs.NewPathNodeFs(gitfs, nil)
	server, _, err := nodefs.MountRoot(gitfsDir, fs.Root(), nil)
	if err != nil {
		logger.Errorf("Failed to mount GitFS on %s due to %s", gitfsDir, err)
		return nil, err
	}
	logger.Debugf("Mount GitFs on %s", gitfsDir)
	gitfs.server = server
	server.SetDebug(config.Debug)

	return gitfs, nil
}

func (gitfs *GitFs) Start() {
	defer func() {
		gitfs.logger.Debugf("FUSE stoping ..., removing %s", gitfs.GitFsDir)
		os.RemoveAll(gitfs.GitFsDir)
	}()
	gitfs.logger.Infof("Start to serve FUSE")
	gitfs.server.Serve()
}

func (gitfs *GitFs) WaitStart() {
	gitfs.server.WaitMount()
}

func (gitfs *GitFs) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("OpenDir: user = %s, repo = %s, path = %s", user, repo, path)
	if user == "" {
		entries, err := ioutil.ReadDir(gitfs.GitRepoDir)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		c := make([]fuse.DirEntry, 0, len(entries))
		for _, entry := range entries {
			c = append(c, fuse.DirEntry{Name: entry.Name(), Mode: uint32(entry.Mode())})
		}
		return c, fuse.OK
	} else if repo == "" {
		entries, err := ioutil.ReadDir(gitfs.GitRepoDir + "/" + user)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		c := make([]fuse.DirEntry, 0, len(entries))
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".git")
			c = append(c, fuse.DirEntry{Name: name, Mode: uint32(entry.Mode())})
		}
		return c, fuse.OK
	} else if path == "" {

	} else {

	}
	return nil, fuse.ENOENT
}

func (gitfs *GitFs) GetAttr(name string, _ *fuse.Context) (*fuse.Attr, fuse.Status) {
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("GetAttr: user = %s, repo = %s, path = %s", user, repo, path)
	if /* user == "" || */ repo == "" {
		fileinfo, err := os.Stat(gitfs.GitRepoDir + "/" + user)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		attr := fuse.ToAttr(fileinfo)
		attr.Mode &= ^uint32(0222)
		return attr, fuse.OK
	} else if path == "" {
		repo += ".git"
		fileinfo, err := os.Stat(gitfs.GitRepoDir + "/" + user + "/" + repo)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		attr := fuse.ToAttr(fileinfo)
		attr.Mode &= ^uint32(0222)
		return attr, fuse.OK
	} else {

	}
	return nil, fuse.ENOENT
}

func splitPath(fullpath string) (string, string, string) {
	paths := strings.SplitN(fullpath, "/", 3)
	switch len(paths) {
	case 1:
		return paths[0], "", ""
	case 2:
		return paths[0], paths[1], ""
	default:
		return paths[0], paths[1], paths[2]
	}
}

func gitfsDir(logger log_driver.Logger) (string, error) {
	dir, err := ioutil.TempDir("", "gitfs")
	if err != nil {
		logger.Errorf("Failed to create a temporary dir due to %s", err)
		return "", err
	}
	return dir, nil
}
