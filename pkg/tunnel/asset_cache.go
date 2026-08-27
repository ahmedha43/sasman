package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CachedAsset holds static asset content and headers for instant Edge serving
type CachedAsset struct {
	Body        []byte
	ContentType string
	ETag        string
	CachedAt    time.Time
}

// AssetCache provides an in-memory high-speed cache for static web assets
type AssetCache struct {
	mu     sync.RWMutex
	assets map[string]*CachedAsset
	maxTTL time.Duration
}

// NewAssetCache creates a new in-memory asset cache with specified max TTL
func NewAssetCache(maxTTL time.Duration) *AssetCache {
	if maxTTL <= 0 {
		maxTTL = 6 * time.Hour
	}
	return &AssetCache{
		assets: make(map[string]*CachedAsset),
		maxTTL: maxTTL,
	}
}

// IsStaticAsset checks if the URL path corresponds to a cacheable static asset
func IsStaticAsset(path string) bool {
	// Strip query parameters
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css", ".js", ".mjs", ".woff", ".woff2", ".ttf", ".eot", ".otf",
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".map":
		return true
	default:
		return false
	}
}

// Get retrieves a cached asset if present and not expired
func (c *AssetCache) Get(path string) (*CachedAsset, bool) {
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	asset, ok := c.assets[path]
	if !ok || asset == nil {
		return nil, false
	}

	if time.Since(asset.CachedAt) > c.maxTTL {
		return nil, false
	}

	return asset, true
}

// Set stores an asset in the cache
func (c *AssetCache) Set(path string, body []byte, contentType string) {
	if len(body) == 0 || !IsStaticAsset(path) {
		return
	}

	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	if contentType == "" {
		ext := filepath.Ext(path)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			switch ext {
			case ".js", ".mjs":
				contentType = "application/javascript; charset=utf-8"
			case ".css":
				contentType = "text/css; charset=utf-8"
			case ".svg":
				contentType = "image/svg+xml"
			case ".woff2":
				contentType = "font/woff2"
			case ".woff":
				contentType = "font/woff"
			default:
				contentType = "application/octet-stream"
			}
		}
	}

	hasher := sha256.New()
	hasher.Write(body)
	etag := `"` + hex.EncodeToString(hasher.Sum(nil))[:16] + `"`

	c.mu.Lock()
	defer c.mu.Unlock()

	c.assets[path] = &CachedAsset{
		Body:        body,
		ContentType: contentType,
		ETag:        etag,
		CachedAt:    time.Now(),
	}
}

// Clear purges all assets from the cache and returns the number of purged items
func (c *AssetCache) Clear() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.assets)
	c.assets = make(map[string]*CachedAsset)
	return count
}

// Stats returns the number of cached assets, estimated memory size, and file list
func (c *AssetCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalBytes int64
	files := make([]map[string]interface{}, 0, len(c.assets))
	for path, asset := range c.assets {
		if asset != nil {
			size := int64(len(asset.Body))
			totalBytes += size
			files = append(files, map[string]interface{}{
				"path":         path,
				"size":         size,
				"content_type": asset.ContentType,
				"cached_ago":   time.Since(asset.CachedAt).Round(time.Second).String(),
			})
		}
	}

	return map[string]interface{}{
		"cached_files": len(c.assets),
		"total_bytes":  totalBytes,
		"max_ttl":      c.maxTTL.String(),
		"files":        files,
	}
}
