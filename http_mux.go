package snack

import (
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
)

// ServeMux do path matching
type ServeMux struct {
	mu sync.RWMutex
	m  map[string]http.Handler
}

// ServeHTTP 实现处理函数
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h, _ := mux.Handler(r)
	h.ServeHTTP(w, r)
}

func (mux *ServeMux) Handler(r *http.Request) (h http.Handler, pattern string) {
	// All other requests have any port stripped and path cleaned
	// before passing to mux.handler.
	host := stripHostPort(r.Host)
	urlPath := cleanPath(r.URL.Path)

	// If the given path is /tree and its handler is not registered,
	// redirect for /tree/.
	if u, ok := mux.redirectToPathSlash(host, urlPath, r.URL); ok {
		return http.RedirectHandler(u.String(), http.StatusMovedPermanently), u.Path
	}

	if urlPath != r.URL.Path {
		_, pattern = mux.handler(host, urlPath)
		u := &url.URL{Path: urlPath, RawQuery: r.URL.RawQuery}
		return http.RedirectHandler(u.String(), http.StatusMovedPermanently), pattern
	}

	return mux.handler(host, r.URL.Path)
}

// redirectToPathSlash determines if the given path needs appending "/" to it.
// This occurs when a handler for path + "/" was already registered, but
// not for path itself. If the path needs appending to, it creates a new
// URL, setting the path to u.Path + "/" and returning true to indicate so.
func (mux *ServeMux) redirectToPathSlash(host, path string, u *url.URL) (*url.URL, bool) {
	mux.mu.RLock()
	shouldRedirect := mux.shouldRedirectRLocked(host, path)
	mux.mu.RUnlock()
	if !shouldRedirect {
		return u, false
	}
	path = path + "/"
	u = &url.URL{Path: path, RawQuery: u.RawQuery}
	return u, true
}

// shouldRedirectRLocked reports whether the given path and host should be redirected to
// path+"/". This should happen if a handler is registered for path+"/" but
// not path -- see comments at ServeMux.
func (mux *ServeMux) shouldRedirectRLocked(host, path string) bool {
	p := []string{path, host + path}

	for _, c := range p {
		if _, exist := mux.m[c]; exist {
			return false
		}
	}

	n := len(path)
	if n == 0 {
		return false
	}
	for _, c := range p {
		if _, exist := mux.m[c+"/"]; exist {
			return path[n-1] != '/'
		}
	}

	return false
}

// handler is the main implementation of Handler.
// The path is known to be in canonical form, except for CONNECT methods.
func (mux *ServeMux) handler(host, path string) (h http.Handler, pattern string) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	if h == nil {
		h, pattern = mux.match(path)
	}
	if h == nil {
		h, pattern = http.NotFoundHandler(), ""
	}
	return
}

// Find a handler on a handler map given a path string.
// Most-specific (longest) pattern wins.
func (mux *ServeMux) match(path string) (h http.Handler, pattern string) {
	// Check for exact match first.
	v, ok := mux.m[path]
	if ok {
		return v, path
	}

	return nil, ""
}

// stripHostPort returns h without any trailing ":<port>".
func stripHostPort(h string) string {
	// If no port on host, return unchanged
	if !strings.Contains(h, ":") {
		return h
	}
	host, _, err := net.SplitHostPort(h)
	if err != nil {
		return h // on error, return unchanged
	}
	return host
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}

// HandleFunc registers the handler function for the given pattern.
func (mux *ServeMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if handler == nil {
		panic("http: nil handler")
	}
	mux.Handle(pattern, http.HandlerFunc(handler))
}

// Handle registers the handler for the given pattern.
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler http.Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("http: invalid pattern")
	}
	if handler == nil {
		panic("http: nil handler")
	}
	if _, exist := mux.m[pattern]; exist {
		panic("http: multiple registrations for " + pattern)
	}

	if mux.m == nil {
		mux.m = make(map[string]http.Handler)
	}
	mux.m[pattern] = handler
}