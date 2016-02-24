package gitfuse

import (
	"hash/crc64"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	conf "github.com/bachue/pages/config"
	"github.com/bachue/pages/gitfuse/cache"
	"github.com/bachue/pages/log_driver"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	libgit2 "gopkg.in/libgit2/git2go.v23"
)

type GitFs struct {
	pathfs.FileSystem
	GitRepoDir string
	GitFsDir   string
	server     *fuse.Server
	logger     log_driver.Logger
	cache      *cache.Cache
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

	gitfs.cache, err = cache.New(1024)
	if err != nil {
		logger.Errorf("Failed to initialize object cache due to %s\n", err)
		return nil, err
	}

	return gitfs, nil
}

func (gitfs *GitFs) Start() {
	defer gitfs.showPanicError()
	defer func() {
		gitfs.logger.Debugf("FUSE stoping ..., removing %s", gitfs.GitFsDir)
		os.RemoveAll(gitfs.GitFsDir)
	}()
	gitfs.logger.Infof("Start to serve FUSE")
	gitfs.server.Serve()
}

func (gitfs *GitFs) WaitStart() {
	defer gitfs.showPanicError()
	gitfs.server.WaitMount()
}

func (gitfs *GitFs) Unmount() error {
	defer gitfs.showPanicError()
	return gitfs.server.Unmount()
}

func (gitfs *GitFs) OpenDir(name string, _ *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	defer gitfs.showPanicError()
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("OpenDir: user = %s, repo = %s, path = %s", user, repo, path)
	if user == "" {
		entries, err := ioutil.ReadDir(gitfs.GitRepoDir)
		if err != nil {
			gitfs.logger.Errorf("Failed to open Git Repo Dir %s due to %s", gitfs.GitRepoDir, err)
			return nil, fuse.ToStatus(err)
		}
		c := make([]fuse.DirEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				c = append(c, fuse.DirEntry{Name: entry.Name(), Mode: uint32(entry.Mode()) ^ 0222})
			}
		}
		return c, fuse.OK
	} else if repo == "" {
		userDir := gitfs.GitRepoDir + "/" + user
		entries, err := ioutil.ReadDir(userDir)
		if err != nil {
			gitfs.logger.Errorf("Failed to open Git User Dir %s due to %s", userDir, err)
			return nil, fuse.ToStatus(err)
		}
		c := make([]fuse.DirEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() && strings.HasSuffix(entry.Name(), ".git") {
				name := strings.TrimSuffix(entry.Name(), ".git")
				c = append(c, fuse.DirEntry{Name: name, Mode: uint32(entry.Mode()) ^ 0222})
			}
		}
		return c, fuse.OK
	}

	repopath := gitfs.GitRepoDir + "/" + user + "/" + repo + ".git"
	entries, status := gitfs.openGitDir(repopath, path)
	return entries, status
}

func (gitfs *GitFs) openGitDir(repoPath string, path string) ([]fuse.DirEntry, fuse.Status) {
	repo, _, _, tree, err := gitfs.getMasterTreeFromRepo(repoPath)
	if err != nil {
		return nil, fuse.EPERM
	}

	if path != "" {
		entry, err := tree.EntryByPath(path)
		if err != nil {
			gitfs.logger.Debugf("Cannot find path %s from tree %s of Git Repository %s due to %s", path, tree.Id().String(), repoPath, err)
			return nil, fuse.ENOENT
		} else if entry.Type == libgit2.ObjectTree {
			tree, err = repo.LookupTree(entry.Id)
			if err != nil {
				gitfs.logger.Errorf("Failed to find tree %s (path = %s) from Git Repository %s", entry.Id, path, repoPath)
				return nil, fuse.EPERM
			}
			defer tree.Free()
		} else {
			gitfs.logger.Debugf("Path %s from tree %s of Git Repository %s is expected to be tree but it's not", path, tree.Id().String(), repoPath)
			return nil, fuse.EINVAL
		}
	}
	count := tree.EntryCount()
	c := make([]fuse.DirEntry, 0, count)
	for i := uint64(0); i < count; i++ {
		entry := tree.EntryByIndex(i)
		if entry == nil {
			gitfs.logger.Errorf("Failed to get tree entry by index %d from tree %s of Git Repository %s", i, tree.Id().String(), repoPath)
			return nil, fuse.EPERM
		} else if entry.Type == libgit2.ObjectTree || entry.Type == libgit2.ObjectBlob {
			c = append(c, fuse.DirEntry{Name: entry.Name, Mode: toFileMode(entry.Filemode)})
		}
	}

	return c, fuse.OK
}

func (gitfs *GitFs) GetAttr(name string, _ *fuse.Context) (attr *fuse.Attr, status fuse.Status) {
	defer gitfs.showPanicError()
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("GetAttr: user = %s, repo = %s, path = %s", user, repo, path)
	if /* user == "" || */ repo == "" {
		dirInfo, err := os.Stat(gitfs.GitRepoDir + "/" + user)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		attr = fuse.ToAttr(dirInfo)
		attr.Mode &= ^uint32(0222)
		return attr, fuse.OK
	}

	repoPath := gitfs.GitRepoDir + "/" + user + "/" + repo + ".git"
	attr, status = gitfs.getGitAttrByPath(repoPath, path)
	return
}

func (gitfs *GitFs) GetXAttr(name string, attr string, _ *fuse.Context) ([]byte, fuse.Status) {
	defer gitfs.showPanicError()
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("GetXAttr: user = %s, repo = %s, path = %s", user, repo, path)
	return nil, fuse.ENODATA
}

func (gitfs *GitFs) ListXAttr(name string, _ *fuse.Context) ([]string, fuse.Status) {
	defer gitfs.showPanicError()
	user, repo, path := splitPath(name)
	gitfs.logger.Debugf("ListXAttr: user = %s, repo = %s, path = %s", user, repo, path)
	return []string{}, fuse.OK
}

func (gitfs *GitFs) getGitAttrByPath(repoPath string, path string) (*fuse.Attr, fuse.Status) {
	repo, _, _, tree, err := gitfs.getMasterTreeFromRepo(repoPath)
	if err != nil {
		return nil, fuse.EPERM
	}

	repoInfo, err := os.Stat(repoPath)
	if err != nil {
		gitfs.logger.Debugf("Failed to Stat %s due to %s", repoPath, err)
		return nil, fuse.ToStatus(err)
	}

	if path == "" {
		attr := fuse.ToAttr(repoInfo)
		attr.Mode &= ^uint32(0222)
		attr.Nlink = 2 + gitfs.treeEntryCount(tree, repoPath)
		return attr, fuse.OK
	}

	entry, err := tree.EntryByPath(path)
	if err != nil {
		gitfs.logger.Debugf("Cannot find path %s from tree %s of Git Repository %s due to %s", path, tree.Id().String(), repoPath, err)
		return nil, fuse.ENOENT
	}
	gitfs.logger.Debugf("Found path %s from tree %s of Git Repository %s", path, tree.Id().String(), repoPath)

	var attr fuse.Attr
	attr.Mode = toFileMode(entry.Filemode)
	if stat, ok := repoInfo.Sys().(*syscall.Stat_t); ok {
		attr.Uid = stat.Uid
		attr.Gid = stat.Gid
		attr.Blksize = uint32(stat.Blksize)
		attr.Rdev = uint32(stat.Rdev)
	}
	attr.Ino = crc64.Checksum(entry.Id[:], crc64.MakeTable(crc64.ECMA))
	now := time.Now()
	attr.SetTimes(&now, &now, &now)

	switch entry.Type {
	case libgit2.ObjectTree:
		tree, err := repo.LookupTree(entry.Id)
		if err != nil {
			gitfs.logger.Errorf("Failed to find tree %s from Git Repository %s", entry.Id, repoPath)
			return nil, fuse.EPERM
		}
		defer tree.Free()
		gitfs.logger.Debugf("Found tree %s of Git Repository %s", tree.Id().String(), repoPath)
		attr.Size = 4096
		attr.Nlink = 2 + gitfs.treeEntryCount(tree, repoPath)
	case libgit2.ObjectBlob:
		blob, err := repo.LookupBlob(entry.Id)
		if err != nil {
			gitfs.logger.Errorf("Failed to find blob %s from Git Repository %s", entry.Id, repoPath)
			return nil, fuse.EPERM
		}
		defer blob.Free()
		gitfs.logger.Debugf("Found blob %s of Git Repository %s", blob.Id().String(), repoPath)
		attr.Nlink = 1
		attr.Size = uint64(blob.Size())
	default:
		gitfs.logger.Debugf("GetAttr: Unsupported object type %s of %s from Git Repository %s", entry.Type.String(), entry.Id, repoPath)
		return nil, fuse.ENOENT
	}
	attr.Blocks = (attr.Size + 511) / 512
	return &attr, fuse.OK
}

func (gitfs *GitFs) getMasterTreeFromRepo(repoPath string) (*libgit2.Repository, *libgit2.Branch, *libgit2.Commit, *libgit2.Tree, error) {
	entry, found := gitfs.cache.Get(repoPath)
	if found {
		gitfs.logger.Debugf("Cache hits on Git Repository %s", repoPath)
		return entry.Repo, entry.Branch, entry.Commit, entry.Tree, nil
	}
	gitfs.logger.Debugf("Cache miss on Git Repository %s", repoPath)
	repo, branch, commit, tree, cleaner, err := gitfs.getMasterTreeFromRepoWithoutCache(repoPath)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	entry = &cache.CacheEntry{Repo: repo, Branch: branch, Commit: commit, Tree: tree, OnClean: cleaner}
	gitfs.cache.Add(repoPath, entry)
	gitfs.logger.Debugf("Cache added for Git Repository %s", repoPath)
	return repo, branch, commit, tree, err
}

func (gitfs *GitFs) getMasterTreeFromRepoWithoutCache(repoPath string) (*libgit2.Repository, *libgit2.Branch, *libgit2.Commit, *libgit2.Tree, func(), error) {
	repo, err := libgit2.OpenRepository(repoPath)
	if err != nil {
		gitfs.logger.Debugf("Failed to open Git Repository %s due to %s", repoPath, err)
		return nil, nil, nil, nil, nil, err
	}
	gitfs.logger.Debugf("Open Git Repository %s", repoPath)
	masterBranch, err := repo.LookupBranch("master", libgit2.BranchLocal)
	if err != nil {
		gitfs.logger.Errorf("Failed to get master branch of Git Repository %s due to %s", repoPath, err)
		repo.Free()
		return nil, nil, nil, nil, nil, err
	}
	gitfs.logger.Debugf("Got master branch of Git Repository %s", repoPath)
	targetCommit, err := repo.LookupCommit(masterBranch.Target())
	if err != nil {
		gitfs.logger.Errorf("Failed to get commit from master branch of Git Repository %s due to %s", repoPath, err)
		masterBranch.Free()
		repo.Free()
		return nil, nil, nil, nil, nil, err
	}
	gitfs.logger.Debugf("Got commit from master branch of Git Repository %s", repoPath)
	targetTree, err := targetCommit.Tree()
	if err != nil {
		gitfs.logger.Errorf("Failed to get tree of commit %s from Git Repository %s due to %s", targetCommit.Id().String(), repoPath, err)
		targetCommit.Free()
		masterBranch.Free()
		repo.Free()
		return nil, nil, nil, nil, nil, err
	}
	gitfs.logger.Debugf("Got tree from master branch of Git Repository %s", repoPath)
	cleaner := func() {
		targetTree.Free()
		targetCommit.Free()
		masterBranch.Free()
		repo.Free()
	}
	return repo, masterBranch, targetCommit, targetTree, cleaner, nil
}

func (gitfs *GitFs) showPanicError() {
	r := recover()
	if r != nil {
		defer gitfs.server.Unmount()
		gitfs.logger.Fatalf("GitFs receives Panic Error: %s", r)
	}
}

func (gitfs *GitFs) treeEntryCount(tree *libgit2.Tree, repoPath string) (count uint32) {
	count = 0
	for i := uint64(0); i < tree.EntryCount(); i++ {
		entry := tree.EntryByIndex(i)
		if entry == nil {
			gitfs.logger.Errorf("Failed to get tree entry by index %d from tree %s of Git Repository %s", i, tree.Id().String(), repoPath)
			return
		}
		if entry.Type == libgit2.ObjectTree {
			count++
		}
	}
	return
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

func toFileMode(filemode libgit2.Filemode) uint32 {
	switch filemode {
	case libgit2.FilemodeTree:
		return fuse.S_IFDIR | 0555
	case libgit2.FilemodeBlob:
		return fuse.S_IFREG | 0444
	case libgit2.FilemodeBlobExecutable:
		return fuse.S_IFREG | 0555
	case libgit2.FilemodeLink:
		return fuse.S_IFLNK | 0777
	default:
		return 0
	}
}
