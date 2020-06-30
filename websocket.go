package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func messageSize(c *config) int {
	return c.width * c.height * 4
}

func (h *handler) getSocket(w http.ResponseWriter, r *http.Request) {
	size := messageSize(h.config)
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  size,
		WriteBufferSize: 1024,
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("Could not upgrade WebSocket:", err)
		return
	}
	e, err := newFFmpegEncoder(h.log, h.config)
	if err != nil {
		ws.Close()
		h.log.Error("Could not create encoder:", err)
		return
	}
	s := &stream{
		ws:       ws,
		done:     make(chan bool),
		config:   h.config,
		log:      h.log,
		hasFrame: make(chan bool, 1),
		encoder:  e,
		length:   -1,
	}
	if h.config.length >= 0.0 {
		s.length = int(math.Round(h.config.length * h.config.framerate))
	}
	go s.read()
	go s.write()
}

type stream struct {
	ws         *websocket.Conn
	done       chan bool
	config     *config
	log        *log.Logger
	frameCount uint32
	hasFrame   chan bool
	encoder    encoder
	length     int
}

func (s *stream) close() {
	s.log.Infoln("Closing socket")
	s.ws.Close()
	if err := s.encoder.close(); err != nil {
		s.log.Errorln("Failed to encode:", err)
	}
}

func (s *stream) error(msg string, err error) {
	s.log.Errorln(msg, err)
}

func (s *stream) read() {
	startTime := time.Now()
	lastStatus := startTime
	var frameCount uint32
	defer close(s.done)
	size := messageSize(s.config)
	s.ws.SetReadLimit(int64(size))
	buf := make([]byte, size+8)
	for {
		s.ws.SetReadDeadline(time.Now().Add(s.config.timeout))
		mt, r, err := s.ws.NextReader()
		if err != nil {
			s.log.Error("NextReader:", err)
			return
		}
		switch mt {
		case websocket.TextMessage:
			s.log.Error("Unexpected text message")
			return
		case websocket.BinaryMessage:
			var pos int
			for pos < len(buf) {
				n, err := r.Read(buf[pos:])
				pos += n
				if err != nil {
					if err == io.EOF {
						break
					}
					s.log.Error("Read:", err)
					return
				}
			}
			if pos == 0 {
				s.log.Infoln("Received end of stream")
				s.log.Infof("Frame count: %d", frameCount)
				return
			}
			if pos != size {
				s.log.Errorf("Got %d bytes, expect %d", pos, size)
				return
			}
			frameCount++
			atomic.StoreUint32(&s.frameCount, frameCount)
			select {
			case s.hasFrame <- true:
			default:
			}
			now := time.Now()
			if now.Sub(lastStatus) > time.Second {
				lastStatus = now
				fps := float64(frameCount) / now.Sub(startTime).Seconds()
				if s.length > 0 {
					s.log.Infof("Frame %d/%d [%.1f%%] (%.2f FPS)",
						frameCount, s.length,
						100.0*float64(frameCount)/float64(s.length), fps)
				} else {
					s.log.Infof("Frame %d (%.2f FPS)", frameCount, fps)
				}
			}
			if err := s.encoder.write(buf[:pos]); err != nil {
				s.log.Error(err)
				return
			}
		default:
			s.log.Errorln("Unknown message type", mt)
			return
		}
	}
}

func (s *stream) write() {
	var frameCount uint32
	defer s.close()
	if err := s.sendStart(); err != nil {
		s.error("sendStart", err)
		return
	}
	t := time.NewTicker(s.config.pingInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.ws.SetWriteDeadline(time.Now().Add(s.config.timeout))
			if err := s.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.error("ping", err)
				return
			}
		case <-s.done:
			return
		case <-s.hasFrame:
			newFrameCount := atomic.LoadUint32(&s.frameCount)
			if frameCount != newFrameCount {
				s.sendAck(newFrameCount)
			}
		}
	}
}

func (s *stream) sendStart() error {
	type startMessage struct {
		Type      string  `json:"type"`
		Width     int     `json:"width"`
		Height    int     `json:"height"`
		FrameRate float64 `json:"framerate"`
		Length    int     `json:"length"`
	}
	m := startMessage{
		Type:      "start",
		Width:     s.config.width,
		Height:    s.config.height,
		FrameRate: s.config.framerate,
		Length:    s.length,
	}
	data, err := json.Marshal(&m)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	s.ws.SetWriteDeadline(time.Now().Add(s.config.timeout))
	err = s.ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return fmt.Errorf("WriteMessage: %w", err)
	}
	return nil
}

func (s *stream) sendAck(count uint32) error {
	type ackMessage struct {
		Type  string `json:"type"`
		Frame uint32 `json:"frame"`
	}
	m := ackMessage{
		Type:  "ack",
		Frame: count,
	}
	data, err := json.Marshal(&m)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	s.ws.SetWriteDeadline(time.Now().Add(s.config.timeout))
	err = s.ws.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return fmt.Errorf("WriteMessage: %w", err)
	}
	return nil
}
