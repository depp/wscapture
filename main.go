package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type config struct {
	listen   string
	appRoot  string
	wsRoot   string
	videoDir string

	width     int
	height    int
	framerate float64
	length    float64

	timeout      time.Duration
	pingInterval time.Duration

	encodeOptions []string
}

func parseSize(size string) (int, int, error) {
	i := strings.IndexByte(size, 'x')
	if i == -1 {
		return 0, 0, errors.New("invalid size: should have format <width>x<height>")
	}
	ws := size[:i]
	hs := size[i+1:]
	w, err := strconv.ParseInt(ws, 10, strconv.IntSize)
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("invalid width %q", ws)
	}
	h, err := strconv.ParseInt(hs, 10, strconv.IntSize)
	if err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("invalid height %q", hs)
	}
	return int(w), int(h), nil
}

func mainE() error {
	var config config
	var size string
	var ec encodeConfig
	flag.StringVar(&config.listen, "http", "localhost:8080", "Listen at address `addr`")
	flag.StringVar(&config.appRoot, "root", ".", "Serve files from `dir`")
	flag.StringVar(&config.videoDir, "videos", "videos", "Directory to store videos")
	flag.StringVar(&size, "size", "640x480", "Video size")
	flag.Float64Var(&config.framerate, "rate", 30.0, "Record at `rate` fps")
	flag.Float64Var(&config.length, "length", -1.0, "Length of video to record, in seconds, or -1 for unlimited")
	flag.DurationVar(&config.timeout, "timeout", 10*time.Second, "Web socket timeout")
	flag.DurationVar(&config.pingInterval, "ping-interval", 20*time.Second, "Web socket ping interval")
	ec.addFlags()
	flag.Parse()
	var err error
	config.width, config.height, err = parseSize(size)
	if err != nil {
		return err
	}
	config.encodeOptions = ec.options()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	config.wsRoot = filepath.Dir(exe)
	l := log.New()
	l.Infoln("Encoding options:", strings.Join(config.encodeOptions, " "))
	h := &handler{
		log:    l,
		config: &config,
	}
	return http.ListenAndServe(config.listen, h)
}

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
