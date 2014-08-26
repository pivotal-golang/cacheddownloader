package cacheddownloader

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fileCache struct {
	cachedPath     string
	maxSizeInBytes int64
	lock           *sync.Mutex
	entries        map[string]fileCacheEntry
	cacheFilePaths map[string]string
}

type fileCacheEntry struct {
	size        int64
	access      time.Time
	cachingInfo CachingInfoType
	filePath    string
}

func NewCache(dir string, maxSizeInBytes int64) *fileCache {
	return &fileCache{
		cachedPath:     dir,
		maxSizeInBytes: maxSizeInBytes,
		lock:           &sync.Mutex{},
		entries:        map[string]fileCacheEntry{},
		cacheFilePaths: map[string]string{},
	}
}

func (c *fileCache) Add(cacheKey string, sourcePath string, size int64, cachingInfo CachingInfoType) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.unsafelyRemoveCacheEntryFor(cacheKey)

	if size > c.maxSizeInBytes {
		//file does not fit in cache...
		return false, nil
	}

	c.makeRoom(size)

	cachePath := filepath.Join(c.cachedPath, filepath.Base(sourcePath))

	err := os.Rename(sourcePath, cachePath)
	if err != nil {
		return false, err
	}

	c.cacheFilePaths[cachePath] = cacheKey
	c.entries[cacheKey] = fileCacheEntry{
		size:        size,
		filePath:    cachePath,
		access:      time.Now(),
		cachingInfo: cachingInfo,
	}

	return true, nil
}

func (c *fileCache) PathForKey(cacheKey string) string {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.entries[cacheKey].filePath
}

func (c *fileCache) RemoveEntry(cacheKey string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.unsafelyRemoveCacheEntryFor(cacheKey)
}

func (c *fileCache) RecordAccess(cacheKey string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	f := c.entries[cacheKey]
	f.access = time.Now()
	c.entries[cacheKey] = f
}

func (c *fileCache) RemoveFileIfUntracked(cacheFilePath string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, isTracked := c.cacheFilePaths[cacheFilePath]
	if !isTracked {
		os.RemoveAll(cacheFilePath)
	}
}

func (c *fileCache) Info(cacheKey string) CachingInfoType {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.entries[cacheKey].cachingInfo
}

func (c *fileCache) makeRoom(size int64) {
	usedSpace := c.usedSpace()
	for c.maxSizeInBytes < usedSpace+size {
		oldestAccessTime, oldestCacheKey := time.Now(), ""
		for ck, f := range c.entries {
			if f.access.Before(oldestAccessTime) {
				oldestCacheKey = ck
				oldestAccessTime = f.access
			}
		}

		usedSpace -= c.entries[oldestCacheKey].size
		c.unsafelyRemoveCacheEntryFor(oldestCacheKey)
	}
}

func (c *fileCache) unsafelyRemoveCacheEntryFor(cacheKey string) {
	fp := c.entries[cacheKey].filePath

	if fp != "" {
		delete(c.cacheFilePaths, fp)
		os.RemoveAll(fp)
	}
	delete(c.entries, cacheKey)
}

func (c *fileCache) usedSpace() int64 {
	space := int64(0)
	for _, f := range c.entries {
		space += f.size
	}
	return space
}