package webrtcc

import (
	"sync"
	"sync/atomic"

	"github.com/pion/webrtc/v3"
)

type (
	whepSession struct {
		videoTrack     *webrtc.TrackLocalStaticRTP
		currentLayer   atomic.Value
		sequenceNumber uint16
		timestamp      uint32
	}

	simulcastLayerResponse struct {
		EncodingId string `json:"encodingId"`
	}
)

type (
	whepLayerRequestJSON struct {
		MediaId    string `json:"mediaId"`
		EncodingId string `json:"encodingId"`
	}
)

type (
	stream struct {
		audioTrack       *webrtc.TrackLocalStaticRTP
		videoTrackLabels []string
		pliChan          chan any
		whepSessionsLock sync.RWMutex
		whepSessions     map[string]*whepSession
	}
)
