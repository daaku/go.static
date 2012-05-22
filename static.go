// Package static provides go.h compatible hashed static asset
// URIs. This allows for providing long lived cache headers for
// resources.
package static

import (
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/nshah/go.h"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"
)

const Path = "/static/"

var (
	cacheMaxAge = flag.Duration(
		"static.max-age",
		time.Hour*87658,
		"Max age to use in the cache headers.")
	fileSystem http.FileSystem
)

type LinkStyle struct {
	HREF  string
	cache h.HTML
}

func (l *LinkStyle) HTML() (h.HTML, error) {
	if l.cache == nil {
		url, err := URL(l.HREF)
		if err != nil {
			return nil, err
		}
		l.cache = &h.LinkStyle{HREF: url}
	}
	return l.cache, nil
}

func notFound(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Cache-Control", "no-cache")
	header.Set("Pragma", "no-cache")
	w.WriteHeader(404)
	w.Write([]byte("static resource not found!"))
}

// Set the resources directory.
func SetDir(publicDir string) {
	fileSystem = http.Dir(publicDir)
}

// Get a hashed URL for a named file.
func URL(name string) (string, error) {
	f, err := fileSystem.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	_, err = h.Write(content)
	if err != nil {
		return "", err
	}
	hex := fmt.Sprintf("%x", h.Sum(nil))
	url := path.Join(Path, hex[:10], name)
	return url, nil
}

// Serves the static resource.
func Handle(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, Path) {
		notFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		notFound(w, r)
		return
	}
	newPath := strings.Join(parts[3:], "/")

	f, err := fileSystem.Open(newPath)
	if err != nil {
		notFound(w, r)
		return
	}
	defer f.Close()

	d, err := f.Stat()
	if err != nil {
		notFound(w, r)
		return
	}

	if d.IsDir() {
		notFound(w, r)
		return
	}

	header := w.Header()
	header.Set(
		"Cache-Control",
		fmt.Sprintf("public, max-age=%d", int(cacheMaxAge.Seconds())))
	http.ServeContent(w, r, d.Name(), d.ModTime(), f)
}
