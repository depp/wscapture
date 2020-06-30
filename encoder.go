package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

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
		"-filter:v", "vflip",
		"-codec:v", "libx264",
		"-preset", "fast",
		"-crf", "18",
		fname,
	)
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
