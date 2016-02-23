package cache

import (
	lru "github.com/hashicorp/golang-lru/simplelru"
	libgit2 "gopkg.in/libgit2/git2go.v23"
)

type Cleaner func()

type Cache struct {
	list *lru.LRU
}

type CacheEntry struct {
	Repo    *libgit2.Repository
	Branch  *libgit2.Branch
	Commit  *libgit2.Commit
	Tree    *libgit2.Tree
	OnClean Cleaner
}

func New(size int) (*Cache, error) {
	list, err := lru.NewLRU(size, clean)
	if err != nil {
		return nil, err
	}
	return &Cache{list: list}, nil
}

func (cache *Cache) Add(key string, value *CacheEntry) bool {
	return cache.list.Add(key, value)
}

func (cache *Cache) Get(key string) (*CacheEntry, bool) {
	valIface, found := cache.list.Get(key)
	if found {
		entry, ok := valIface.(*CacheEntry)
		return entry, ok
	}
	return nil, false
}

func (cache *Cache) Remove(key string) bool {
	return cache.list.Remove(key)
}

func clean(_ interface{}, value interface{}) {
	entry, ok := value.(*CacheEntry)
	if ok {
		if entry.OnClean != nil {
			entry.OnClean()
		}
	}
}
