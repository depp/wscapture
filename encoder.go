package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type encodeConfig struct {
	codec     string
	crf       int
	preset    string
	profile   string
	pixFormat string
	tune      string
	extra     string
}

func (c *encodeConfig) addFlags() {
	flag.StringVar(&c.codec, "codec", "libx264", "FFmpeg codec")
	flag.IntVar(&c.crf, "crf", -1, "CRF (lower numbers are higher quality)")
	flag.StringVar(&c.preset, "preset", "", "encoder preset")
	flag.StringVar(&c.profile, "profile", "", "encoder profile")
	flag.StringVar(&c.pixFormat, "pix_fmt", "", "video pixel format")
	flag.StringVar(&c.tune, "tune", "", "encoder tuning")
	flag.StringVar(&c.extra, "encode_options", "", "encoder options")
}

func (c *encodeConfig) options() []string {
	var r []string
	if c.codec == "libx264" {
		if c.crf == 0 {
			c.crf = 18
		}
		if c.preset == "" {
			c.preset = "fast"
		}
	}
	r = append(r, "-codec:v", c.codec)
	if c.crf != -1 {
		r = append(r, "-crf", strconv.Itoa(c.crf))
	}
	if c.preset != "" {
		r = append(r, "-preset", c.preset)
	}
	if c.profile != "" {
		r = append(r, "-profile:v", c.profile)
	}
	if c.tune != "" {
		r = append(r, "-tune:v", c.tune)
	}
	r = append(r, strings.Fields(c.extra)...)
	return r
}

const fileNameFormat = "2006-01-02T15-04-05"

type encoder interface {
	write(buf []byte) error
	close() error
}

type pipeEncoder struct {
	log   *log.Logger
	cmd   *exec.Cmd
	pipe  *os.File
	fpath string
	fname string
}

func newFFmpegEncoder(log *log.Logger, c *config) (e *pipeEncoder, err error) {
	if err := os.MkdirAll(c.videoDir, 0777); err != nil {
		return nil, err
	}
	name := time.Now().Format(fileNameFormat)
	const extension = ".mkv"
	fname := name + extension
	fpath := filepath.Join(c.videoDir, fname)
	switch _, err := os.Stat(fpath); {
	case err == nil:
		return nil, fmt.Errorf("file exists: %q", fpath)
	case os.IsNotExist(err):
	default:
		return nil, err
	}
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			w.Close()
			r.Close()
		}
	}()
	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-f", "rawvideo",
		"-pix_fmt", "rgb0",
		"-r", strconv.FormatFloat(c.framerate, 'f', -1, 64),
		"-s", fmt.Sprintf("%dx%d", c.width, c.height),
		"-i", "pipe:3",
	)
	cmd.Args = append(cmd.Args, c.encodeOptions...)
	cmd.Args = append(cmd.Args, fname)
	cmd.Dir = c.videoDir
	cmd.ExtraFiles = []*os.File{r}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &pipeEncoder{
		log:   log,
		cmd:   cmd,
		pipe:  w,
		fpath: fpath,
		fname: fname,
	}, nil
}

func (e *pipeEncoder) write(buf []byte) error {
	for len(buf) != 0 {
		n, err := e.pipe.Write(buf)
		buf = buf[n:]
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *pipeEncoder) close() error {
	e1 := e.pipe.Close()
	e2 := e.cmd.Wait()
	if e2 != nil {
		return e2
	}
	if e1 != nil {
		return e1
	}
	e.log.Infoln("Wrote", e.fname)
	return nil
}
