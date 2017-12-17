package proxy

import (
	"fmt"
	"io"
	"time"
)

type Direction uint8

const (
	Upstream Direction = iota
	Downstream
	NumDirections
)

type StreamChunk struct {
	Data      []byte
	Timestamp time.Time
}

type ChanWriter struct {
	output chan<- *StreamChunk
}

func NewChanWriter(output chan<- *StreamChunk) *ChanWriter {
	return &ChanWriter{output}
}

func (c *ChanWriter) Write(buf []byte) (int, error) {
	packet := &StreamChunk{make([]byte, len(buf)), time.Now()}
	copy(packet.Data, buf)
	c.output <- packet
	return len(buf), nil
}

func (c *ChanWriter) Close() error {
	close(c.output)
	return nil
}

type ChanReader struct {
	input     <-chan *StreamChunk
	interrupt <-chan struct{}
	buffer    []byte
}

var ErrInterrupted = fmt.Errorf("read interrupted by channel")

func NewChanReader(input <-chan *StreamChunk) *ChanReader {
	return &ChanReader{input, make(chan struct{}), []byte{}}
}

func (c *ChanReader) SetInterrupt(interrupt <-chan struct{}) {
	c.interrupt = interrupt
}

func (c *ChanReader) Read(out []byte) (int, error) {
	if c.buffer == nil {
		return 0, io.EOF
	}
	n := copy(out, c.buffer)
	c.buffer = c.buffer[n:]
	if len(out) <= len(c.buffer) {
		return n, nil
	} else if n > 0 {
		select {
		case p := <-c.input:
			if p == nil {
				c.buffer = nil
				if n > 0 {
					return n, nil
				}
				return 0, io.EOF
			}
			n2 := copy(out[n:], p.Data)
			c.buffer = p.Data[n2:]
			return n + n2, nil
		default:
			return n, nil
		}
	}
	var p *StreamChunk
	select {
	case p = <-c.input:
	case <-c.interrupt:
		c.buffer = c.buffer[:0]
		return n, ErrInterrupted
	}
	if p == nil {
		c.buffer = nil
		return 0, io.EOF
	}
	n2 := copy(out[n:], p.Data)
	c.buffer = p.Data[n2:]
	return n + n2, nil
}
