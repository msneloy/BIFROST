package webrtc

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"golang.org/x/sys/unix"
)

type RTPReceiver struct {
	videoConn  *net.UDPConn
	audioConn  *net.UDPConn
	videoTrack *webrtc.TrackLocalStaticRTP
	audioTrack *webrtc.TrackLocalStaticRTP
	videoPkts  atomic.Uint64
	audioPkts  atomic.Uint64
}

// listenUDPWithReuse binds a UDP port with SO_REUSEPORT so stale sockets
// from crashed/killed processes don't block startup.
func listenUDPWithReuse(port int) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}

	addr := fmt.Sprintf(":%d", port)
	// Retry a few times in case the old socket is still tearing down.
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		conn, err := lc.ListenPacket(context.Background(), "udp", addr)
		if err == nil {
			udpConn, ok := conn.(*net.UDPConn)
			if !ok {
				conn.Close()
				return nil, fmt.Errorf("unexpected conn type %T", conn)
			}
			return udpConn, nil
		}
		lastErr = err
		log.Printf("[WebRTC] Port %d not ready, retrying... (%v)", port, err)
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("failed to bind UDP port %d after retries: %w", port, lastErr)
}

func NewRTPReceiver(videoPort, audioPort int) (*RTPReceiver, error) {
	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", "bifrost",
	)
	if err != nil {
		return nil, err
	}

	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "bifrost",
	)
	if err != nil {
		return nil, err
	}

	videoConn, err := listenUDPWithReuse(videoPort)
	if err != nil {
		return nil, err
	}

	audioConn, err := listenUDPWithReuse(audioPort)
	if err != nil {
		videoConn.Close()
		return nil, err
	}

	return &RTPReceiver{
		videoConn:  videoConn,
		audioConn:  audioConn,
		videoTrack: videoTrack,
		audioTrack: audioTrack,
	}, nil
}

func (r *RTPReceiver) Start(ctx context.Context) error {
	go r.readLoop(ctx, r.videoConn, r.videoTrack, &r.videoPkts, "video")
	go r.readLoop(ctx, r.audioConn, r.audioTrack, &r.audioPkts, "audio")
	return nil
}

// AudioPktCount returns the number of audio RTP packets received.
func (r *RTPReceiver) AudioPktCount() uint64 {
	return r.audioPkts.Load()
}

// VideoPktCount returns the number of video RTP packets received.
func (r *RTPReceiver) VideoPktCount() uint64 {
	return r.videoPkts.Load()
}

func (r *RTPReceiver) readLoop(ctx context.Context, conn *net.UDPConn, track *webrtc.TrackLocalStaticRTP, counter *atomic.Uint64, name string) {
	buf := make([]byte, 1500) // MTU-sized buffer
	pktCount := 0
	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return
		default:
		}

		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buf[:n]); err != nil {
			log.Printf("[WebRTC] RTP parse error (%s): %v", name, err)
			continue
		}

		pktCount++
		counter.Add(1)
		if pktCount == 50 {
			log.Printf("[WebRTC] RTP %s: first 50 packets received", name)
		}
		// Only log errors — normal packet flow pollutes the TUI
		if err := track.WriteRTP(packet); err != nil {
			if pktCount <= 3 {
				log.Printf("[WebRTC] RTP %s WriteRTP error: %v", name, err)
			}
			continue
		}
	}
}

func (r *RTPReceiver) VideoTrack() *webrtc.TrackLocalStaticRTP {
	return r.videoTrack
}

func (r *RTPReceiver) AudioTrack() *webrtc.TrackLocalStaticRTP {
	return r.audioTrack
}

func (r *RTPReceiver) Stop() {
	if r.videoConn != nil {
		r.videoConn.Close()
	}
	if r.audioConn != nil {
		r.audioConn.Close()
	}
}
