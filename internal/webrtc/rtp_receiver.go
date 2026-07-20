package webrtc

import (
	"context"
	"log"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type RTPReceiver struct {
	videoConn  *net.UDPConn
	audioConn  *net.UDPConn
	videoTrack *webrtc.TrackLocalStaticRTP
	audioTrack *webrtc.TrackLocalStaticRTP
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

	videoAddr := &net.UDPAddr{Port: videoPort}
	videoConn, err := net.ListenUDP("udp", videoAddr)
	if err != nil {
		return nil, err
	}

	audioAddr := &net.UDPAddr{Port: audioPort}
	audioConn, err := net.ListenUDP("udp", audioAddr)
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
	go r.readLoop(ctx, r.videoConn, r.videoTrack, "video")
	go r.readLoop(ctx, r.audioConn, r.audioTrack, "audio")
	return nil
}

func (r *RTPReceiver) readLoop(ctx context.Context, conn *net.UDPConn, track *webrtc.TrackLocalStaticRTP, name string) {
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
		if pktCount <= 3 || pktCount%500 == 0 {
			log.Printf("[WebRTC] RTP %s packet #%d: seq=%d ts=%d size=%d", name, pktCount, packet.SequenceNumber, packet.Timestamp, n)
		}

		if err := track.WriteRTP(packet); err != nil {
			if pktCount <= 10 || pktCount%100 == 0 {
				log.Printf("[WebRTC] RTP %s WriteRTP error (#%d): %v", name, pktCount, err)
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
