package webrtc

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/pion/rtp"
)

// Relay reads RTP packets from a UDP listener and writes them to the SFU tracks.
type Relay struct {
	sfu        *SFU
	videoPort  int
	audioPort  int
	videoConn  *net.UDPConn
	audioConn  *net.UDPConn
	done       chan struct{}
}

// NewRelay creates a new RTP relay.
func NewRelay(sfu *SFU, videoPort, audioPort int) *Relay {
	return &Relay{
		sfu:       sfu,
		videoPort: videoPort,
		audioPort: audioPort,
		done:      make(chan struct{}),
	}
}

// Start begins listening for RTP packets on the specified UDP ports.
func (r *Relay) Start() error {
	// Video RTP listener
	videoAddr := &net.UDPAddr{Port: r.videoPort}
	videoConn, err := net.ListenUDP("udp", videoAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on video port %d: %w", r.videoPort, err)
	}
	r.videoConn = videoConn
	log.Printf("RTP video relay listening on :%d", r.videoPort)

	// Audio RTP listener
	audioAddr := &net.UDPAddr{Port: r.audioPort}
	audioConn, err := net.ListenUDP("udp", audioAddr)
	if err != nil {
		videoConn.Close()
		return fmt.Errorf("failed to listen on audio port %d: %w", r.audioPort, err)
	}
	r.audioConn = audioConn
	log.Printf("RTP audio relay listening on :%d", r.audioPort)

	go r.readLoop(videoConn, r.sfu.GetVideoTrack(), "video")
	go r.readLoop(audioConn, r.sfu.GetAudioTrack(), "audio")

	return nil
}

func (r *Relay) readLoop(conn *net.UDPConn, track interface{ Write([]byte) (int, error) }, label string) {
	buf := make([]byte, 1500) // MTU-sized buffer
	for {
		select {
		case <-r.done:
			return
		default:
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if err == io.EOF {
					return
				}
				log.Printf("RTP %s read error: %v", label, err)
				continue
			}

			// Parse RTP packet to validate
			var pkt rtp.Packet
			if err := pkt.Unmarshal(buf[:n]); err != nil {
				continue // Skip malformed packets
			}

			// Write raw RTP packet to the track
			if _, err := track.Write(buf[:n]); err != nil {
				log.Printf("RTP %s write error: %v", label, err)
			}
		}
	}
}

// Stop shuts down the relay.
func (r *Relay) Stop() {
	close(r.done)
	if r.videoConn != nil {
		r.videoConn.Close()
	}
	if r.audioConn != nil {
		r.audioConn.Close()
	}
}
