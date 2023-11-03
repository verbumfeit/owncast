package webrtcc

import (
	"time"

	"github.com/owncast/owncast/models"
	"github.com/pion/webrtc/v3"
)

func setCurrentBroadcasterInfo(track *webrtc.TrackRemote) {
	// TODO: Get Stream Metadata (this creates a empty mock tag object for now (and currently isn't used at all))
	// tag := new(flvio.Tag)
	// data, err := getInboundDetailsFromMetadata(tag.DebugFields())
	// if err != nil {
	// 	log.Traceln("Unable to parse inbound broadcaster details:", err)
	// }

	broadcaster := models.Broadcaster{
		RemoteAddr: "123.1.1.1", // TODO: Set to Remote IP
		Time:       time.Now(),
		StreamDetails: models.InboundStreamDetails{
			Width:          200,
			Height:         300,
			VideoBitrate:   2222,
			VideoCodec:     getVideoCodec(track),
			VideoFramerate: 30,
			AudioBitrate:   128,
			AudioCodec:     getAudioCodec(track),
			Encoder:        "deine mudder",
			VideoOnly:      false,
		},
	}

	_setBroadcaster(broadcaster)
}
