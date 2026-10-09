package asp

import (
	"os"
	"sync"
	"time"

	"g3pix.com.br/axonasp/v2/vbscript"
)

type cachedSource struct {
	modified time.Time
	size     int64
	text     string
	regions  []vbscript.ASPRegion
}

// SourceCache is scoped to a run, never global. Reads check file metadata;
// snapshots and scan results are immutable and safe for concurrent page checks.
// Compiler symbols/checkers are deliberately not shared across entry scopes.
type SourceCache struct {
	mu    sync.Mutex
	files map[string]cachedSource
}

func NewSourceCache() *SourceCache { return &SourceCache{files: map[string]cachedSource{}} }

func (c *SourceCache) read(file string) (string, error) {
	info, err := os.Stat(file)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if source, ok := c.files[file]; ok && source.size == info.Size() && source.modified.Equal(info.ModTime()) {
		return source.text, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	text := string(data)
	c.files[file] = cachedSource{modified: info.ModTime(), size: info.Size(), text: text}
	return text, nil
}

func (c *SourceCache) scan(file, text string) ([]vbscript.ASPRegion, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	source, ok := c.files[file]
	if ok && source.text == text && source.regions != nil {
		return source.regions, nil
	}
	regions, err := vbscript.ScanASP(text)
	if err == nil && ok && source.text == text {
		source.regions = regions
		c.files[file] = source
	}
	return regions, err
}
