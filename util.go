package livepkg

import (
	"path"
	"path/filepath"
	"strings"
)

// safely convert path into filename in dir
func urlfile(dir, url string) string {
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	url = path.Clean(url)[1:]
	rel := filepath.FromSlash(url)
	return filepath.Join(dir, rel)
}
