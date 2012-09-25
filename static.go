// Package static provides go.h compatible hashed static asset
// URIs. This allows for providing long lived cache headers for
// resources which change as their content changes.
package static

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/daaku/go.h"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const Path = "/static/"

var (
	cacheMaxAge = flag.Duration(
		"static.max-age",
		time.Hour*87658,
		"Max age to use in the cache headers.")
	cacheEnable = flag.Bool(
		"static.cache",
		true,
		"use in memory cache for static resources")
	fileSystem http.FileSystem
	cache      = make(map[string]cacheEntry)
)

type cacheEntry struct {
	Content []byte
	ModTime time.Time
}

type LinkStyle struct {
	HREF  []string
	cache h.HTML
}

func (l *LinkStyle) HTML() (h.HTML, error) {
	if !*cacheEnable || l.cache == nil {
		url, err := CombinedURL(l.HREF)
		if err != nil {
			return nil, err
		}
		l.cache = &h.LinkStyle{HREF: url}
	}
	return l.cache, nil
}

type Script struct {
	Src   []string
	cache h.HTML
}

func (l *Script) HTML() (h.HTML, error) {
	if !*cacheEnable || l.cache == nil {
		url, err := CombinedURL(l.Src)
		if err != nil {
			return nil, err
		}
		l.cache = &h.Script{Src: url}
	}
	return l.cache, nil
}

// For github.com/daaku/go.h.js.loader compatibility.
func (l *Script) URLs() []string {
	url, err := CombinedURL(l.Src)
	if err != nil {
		panic(err)
	}
	return []string{url}
}

// For github.com/daaku/go.h.js.loader compatibility.
func (l *Script) Script() string {
	return ""
}

type Img struct {
	ID    string
	Class string
	Style string
	Src   string
	Alt   string
	cache h.HTML
}

func (i *Img) HTML() (h.HTML, error) {
	if !*cacheEnable || i.cache == nil {
		src, err := URL(i.Src)
		if err != nil {
			return nil, err
		}
		i.cache = &h.Node{
			Tag:         "img",
			SelfClosing: true,
			Attributes: h.Attributes{
				"id":    i.ID,
				"class": i.Class,
				"style": i.Style,
				"src":   src,
				"alt":   i.Alt,
			},
		}
	}
	return i.cache, nil
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

// Get a hashed URL for a single file.
func URL(name string) (string, error) {
	return CombinedURL([]string{name})
}

// Get a hashed combined URL for all named files.
func CombinedURL(names []string) (string, error) {
	h := md5.New()
	var ce cacheEntry
	for _, name := range names {
		f, err := fileSystem.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return "", err
		}
		modTime := stat.ModTime()
		if ce.ModTime.Before(modTime) {
			ce.ModTime = modTime
		}

		content, err := ioutil.ReadAll(f)
		if err != nil {
			return "", err
		}
		ce.Content = append(ce.Content, content...)
		_, err = h.Write(content)
		if err != nil {
			return "", err
		}
	}
	hex := fmt.Sprintf("%x", h.Sum(nil))
	hexS := hex[:10]
	url := path.Join(Path, hexS, joinBasenames(names))
	cache[hexS] = ce
	return url, nil
}

func joinBasenames(names []string) string {
	basenames := make([]string, len(names))
	for i, name := range names {
		basenames[i] = filepath.Base(name)
	}
	return strings.Join(basenames, "-")
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

	ce, ok := cache[parts[2]]
	if !ok {
		notFound(w, r)
		return
	}

	header := w.Header()
	header.Set(
		"Cache-Control",
		fmt.Sprintf("public, max-age=%d", int(cacheMaxAge.Seconds())))
	http.ServeContent(w, r, path, ce.ModTime, bytes.NewReader(ce.Content))
}
