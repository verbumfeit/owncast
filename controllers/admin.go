package controllers

import (
	"net/http"

	"github.com/owncast/owncast/core/webrtcc" // TODO: This needs to be replaced with github package

	"github.com/owncast/owncast/core/rtmp"
)

// DisconnectInboundConnection will force-disconnect an inbound stream.
func DisconnectInboundConnection(w http.ResponseWriter, r *http.Request) {
	rtmp.Disconnect()
	webrtcc.DeleteStream()
	w.WriteHeader(http.StatusOK)
}
