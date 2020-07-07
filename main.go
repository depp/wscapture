package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

	format        string
	encodeOptions []string
}

var presetSizes = [...][2]int{
	{426, 240},
	{640, 360},
	{854, 480},
	{1280, 720},
	{1920, 1080},
	{2560, 1440},
	{3840, 2160},
}

var errInvalidSize = errors.New("invalid size: should have format <width>x<height> or <height>p")

func parseSize(size string) (int, int, error) {
	i := strings.IndexByte(size, 'x')
	if i == -1 {
		if strings.HasSuffix(size, "p") {
			ws := size[:len(size)-1]
			w, err := strconv.ParseInt(ws, 10, strconv.IntSize)
			if err != nil || w < 0 {
				return 0, 0, errInvalidSize
			}
			for _, s := range presetSizes {
				if s[1] == int(w) {
					return s[0], s[1], nil
				}
			}
			var r []string
			for _, s := range presetSizes {
				r = append(r, strconv.Itoa(s[1])+"p")
			}
			return 0, 0, fmt.Errorf("invalid size %q: valid presets are %s", size, strings.Join(r, ", "))
		}
		return 0, 0, errInvalidSize
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
	flag.StringVar(&config.format, "format", "mkv", "Video container format")
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
	lg := log.New()
	lg.Infof("Size: %dx%d", config.width, config.height)
	lg.Infoln("Encoding options:", strings.Join(config.encodeOptions, " "))
	l, err := net.Listen("tcp", config.listen)
	if err != nil {
		return err
	}
	lg.Infof("Listening at %s", &url.URL{
		Scheme: "http",
		Host:   config.listen,
		Path:   "/",
	})
	h := handler{
		log:    lg,
		config: &config,
	}
	s := http.Server{Handler: &h}
	return s.Serve(l)
}

func main() {
	if err := mainE(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
