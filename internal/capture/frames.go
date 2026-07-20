package capture

import (
	"context"
	"io"
	"log"
)

const (
	jpegSOI  = 0xFF
	jpegSOI2 = 0xD8
	jpegEOI  = 0xFF
	jpegEOI2 = 0xD9
)

type FrameSplitter struct {
	reader    io.Reader
	muxBuffer *MuxBuffer
}

func NewFrameSplitter(r io.Reader, mb *MuxBuffer) *FrameSplitter {
	return &FrameSplitter{
		reader:    r,
		muxBuffer: mb,
	}
}

func (fs *FrameSplitter) Run(ctx context.Context) error {
	buf := make([]byte, 256*1024)
	var leftover []byte

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := fs.reader.Read(buf)
		if n > 0 {
			data := append(leftover, buf[:n]...)
			leftover = fs.processFrames(data)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("[!] FrameSplitter read error: %v", err)
			return err
		}
	}
}

func (fs *FrameSplitter) processFrames(data []byte) []byte {
	i := 0
	for i < len(data)-1 {
		if data[i] == jpegSOI && data[i+1] == jpegSOI2 {
			start := i
			j := i + 2
			for j < len(data)-1 {
				if data[j] == jpegEOI && data[j+1] == jpegEOI2 {
					frame := make([]byte, j+2-start)
					copy(frame, data[start:j+2])
					fs.muxBuffer.PublishVideo(frame)
					i = j + 2
					goto next
				}
				j++
			}
			return data[start:]
		}
		i++
		continue
	next:
	}

	if i < len(data) {
		return data[i:]
	}
	return nil
}
