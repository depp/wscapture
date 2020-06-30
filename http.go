package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type handler struct {
	log    *log.Logger
	config *config

	m sync.Mutex
}

func (h *handler) logResponse(r *http.Request, code int) {
	level := log.InfoLevel
	if code >= 400 {
		if code >= 500 {
			level = log.ErrorLevel
		} else {
			level = log.WarnLevel
		}
	}
	h.log.Logln(level, code, r.URL)
}

func (h *handler) serveError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	h.logResponse(r, code)
	http.Error(w, msg, code)
}

func (h *handler) notFound(w http.ResponseWriter, r *http.Request) {
	h.serveError(w, r, http.StatusNotFound, fmt.Sprintf("Not found: %q", r.URL))
}

func (h *handler) internalError(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error(err)
	h.serveError(w, r, http.StatusInternalServerError, fmt.Sprintf("Internal error: %v", err))
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	path := r.URL.EscapedPath()
	if len(path) < 1 || path[0] != '/' {
		h.notFound(w, r)
		return
	}
	parts := strings.Split(path[1:], "/")
	if parts[0] == "__wscapture__" {
		if len(parts) != 2 {
			h.notFound(w, r)
			return
		}
		switch parts[1] {
		case "socket":
			h.getSocket(w, r)
		case "script.js":
			h.handleWsFile(w, r, "wscapture.bundle.js", true)
		case "module.js":
			h.handleWsFile(w, r, "wscapture.js", false)
		default:
			h.notFound(w, r)
		}
	} else {
		h.handleAppFile(w, r, parts)
	}
}

func (h *handler) getFile(name string, doBuild bool) *os.File {
	if doBuild {
		h.m.Lock()
		defer h.m.Unlock()

		cmd := exec.Command("make", name)
		cmd.Dir = h.config.wsRoot
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			h.log.Errorln("Could not build target:", name)
			os.Stderr.Write(buf.Bytes())
			return nil
		}
	}
	fp, err := os.Open(filepath.Join(h.config.wsRoot, name))
	if err != nil {
		h.log.Errorln("Could not open file:", err)
		return nil
	}
	return fp
}

func (h *handler) handleWsFile(w http.ResponseWriter, r *http.Request, name string, doBuild bool) {
	fp := h.getFile(name, doBuild)
	defer fp.Close()
	st, err := fp.Stat()
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	h.logResponse(r, http.StatusOK)
	http.ServeContent(w, r, name, st.ModTime(), fp)
}

func (h *handler) handleAppFile(w http.ResponseWriter, r *http.Request, parts []string) {
	fparts := make([]string, 0, len(parts)+1)
	fparts = append(fparts, h.config.appRoot)
	for _, part := range parts {
		fpart, err := url.PathUnescape(part)
		if err != nil {
			h.notFound(w, r)
			return
		}
		if fpart == "" || fpart == "." || fpart == ".." {
			h.notFound(w, r)
			return
		}
		for _, c := range []byte(fpart) {
			if c < 0x20 || c == '/' {
				h.notFound(w, r)
				return
			}
		}
		fparts = append(fparts, fpart)
	}
	fpath := filepath.Join(fparts...)
	fp, err := os.Open(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			h.notFound(w, r)
			return
		}
	}
	defer fp.Close()
	st, err := fp.Stat()
	if err != nil {
		h.internalError(w, r, err)
		return
	}
	h.logResponse(r, http.StatusOK)
	name := fparts[len(fparts)-1]
	http.ServeContent(w, r, name, st.ModTime(), fp)
}
