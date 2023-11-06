package webrtcc

import (
	"errors"
	"io"
	"log"
	"strings"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

func audioWriter(remoteTrack *webrtc.TrackRemote, audioTrack *webrtc.TrackLocalStaticRTP) {
	rtpBuf := make([]byte, 1500)
	for {
		rtpRead, _, err := remoteTrack.Read(rtpBuf)
		switch {
		case errors.Is(err, io.EOF):
			return
		case err != nil:
			log.Println(err)
			return
		}

		if _, writeErr := audioTrack.Write(rtpBuf[:rtpRead]); writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
			log.Println(writeErr)
			return
		}
	}
}

func videoWriter(remoteTrack *webrtc.TrackRemote, stream *stream, peerConnection *webrtc.PeerConnection, s *stream) {
	id := remoteTrack.RID()
	if id == "" {
		id = videoTrackLabelDefault
	}

	if err := addTrack(s, id); err != nil {
		log.Println(err)
		return
	}

	go func() {
		for range stream.pliChan {
			if sendErr := peerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(remoteTrack.SSRC()),
				},
			}); sendErr != nil {
				return
			}
		}
	}()

	isAV1 :=
		strings.Contains(
			strings.ToLower(webrtc.MimeTypeAV1),
			strings.ToLower(remoteTrack.Codec().RTPCodecCapability.MimeType),
		)

	rtpBuf := make([]byte, 1500)
	rtpPkt := &rtp.Packet{}
	lastTimestamp := uint32(0)
	for {
		rtpRead, _, err := remoteTrack.Read(rtpBuf)
		switch {
		case errors.Is(err, io.EOF):
			return
		case err != nil:
			log.Println(err)
			return
		}

		if err = rtpPkt.Unmarshal(rtpBuf[:rtpRead]); err != nil {
			log.Println(err)
			return
		}

		timeDiff := rtpPkt.Timestamp - lastTimestamp
		if lastTimestamp == 0 {
			timeDiff = 0
		}
		lastTimestamp = rtpPkt.Timestamp

		s.whepSessionsLock.RLock()
		for i := range s.whepSessions {
			s.whepSessions[i].sendVideoPacket(rtpPkt, id, timeDiff, isAV1)
		}
		s.whepSessionsLock.RUnlock()
	}
}

func GetAllStreams() (out []string) {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	for s := range streamMap {
		out = append(out, s)
	}

	return
}
