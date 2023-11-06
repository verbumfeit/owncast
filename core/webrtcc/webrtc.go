package webrtcc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/owncast/owncast/core/data"

	log "github.com/sirupsen/logrus"

	"github.com/owncast/owncast/models"

	"github.com/pion/ice/v2"
	"github.com/pion/webrtc/v3"
)

const (
	videoTrackLabelDefault = "default"
)

var (
	streamMap        map[string]*stream
	streamMapLock    sync.Mutex
	apiWhip, apiWhep *webrtc.API
)

var _hasInboundWebRTCConnection = false

var (
	_webrtcConnection *webrtc.PeerConnection
)

var (
	_setStreamAsConnected func(*webrtc.PeerConnection)
	_setBroadcaster       func(models.Broadcaster)
)

func getPublicIP() string {
	req, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		log.Fatal(err)
	}
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Fatal(err)
	}

	ip := struct {
		Query string
	}{}
	if err = json.Unmarshal(body, &ip); err != nil {
		log.Fatal(err)
	}

	if ip.Query == "" {
		log.Fatal("Query entry was not populated")
	}

	return ip.Query
}

func createSettingEngine(isWHIP bool, udpMuxCache map[int]*ice.MultiUDPMuxDefault) (settingEngine webrtc.SettingEngine) {
	var (
		NAT1To1IPs []string
		udpMuxPort int
		udpMuxOpts []ice.UDPMuxFromPortOption
		err        error
	)

	// TODO: Are more configurations needed? (see broadcast-box code)
	settingEngine.SetNAT1To1IPs(NAT1To1IPs, webrtc.ICECandidateTypeHost)

	// if os.Getenv("INTERFACE_FILTER") != "" {
	// 	settingEngine.SetInterfaceFilter(func(i string) bool {
	// 		return i == os.Getenv("INTERFACE_FILTER")
	// 	})
	// }

	// udpPort := data.GetWebRTCPortNumber()
	udpMuxPort = 50137 // TODO: Get udpMuxPort from config
	
	udpMux, ok := udpMuxCache[udpMuxPort]
	if !ok {
		if udpMux, err = ice.NewMultiUDPMuxFromPort(udpMuxPort, udpMuxOpts...); err != nil {
			log.Fatal(err)
		}
		udpMuxCache[udpMuxPort] = udpMux
	}

	settingEngine.SetICEUDPMux(udpMux)

	// if os.Getenv("TCP_MUX_ADDRESS") != "" {
	// 	tcpAddr, err := net.ResolveTCPAddr("udp", os.Getenv("TCP_MUX_ADDRESS"))
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}

	// 	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}

	// 	settingEngine.SetICETCPMux(webrtc.NewICETCPMux(nil, tcpListener, 8))
	// }
	return
}

func WHIP(offer, streamKey string) (string, error) {
	peerConnection, err := apiWhip.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return "", err
	}

	// Unlock streamMap and get new stream
	streamMapLock.Lock()
	defer streamMapLock.Unlock()
	stream, err := getStream(streamKey)
	if err != nil {
		return "", err
	}

	// Handle audio (if available) and video tracks
	peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		if strings.HasPrefix(remoteTrack.Codec().RTPCodecCapability.MimeType, "audio") {
			audioWriter(remoteTrack, stream.audioTrack)
		} else {
			videoWriter(remoteTrack, stream, peerConnection, stream)
		}
		// Update broadcaster as we have the needed information here
		setCurrentBroadcasterInfo(remoteTrack)
	})

	// Handle ICE connection fail (close connection and delete stream)
	peerConnection.OnICEConnectionStateChange(func(i webrtc.ICEConnectionState) {
		if i == webrtc.ICEConnectionStateFailed {
			if err := peerConnection.Close(); err != nil {
				log.Println(err)
			}
			handleDisconnect(peerConnection)
		}
	})

	// Handle surprising disconnect
	// peerConnection.OnConnectionStateChange(func(i webrtc.PeerConnectionState) {
	// 	if i == webrtc.PeerConnectionStateFailed || i == webrtc.PeerConnectionStateClosed  {
	// 		if err := peerConnection.Close(); err != nil {
	// 			log.Println(err)
	// 		}
	// 		handleDisconnect(peerConnection)
	// 	}
	// })

	if err := peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  string(offer),
		Type: webrtc.SDPTypeOffer,
	}); err != nil {
		return "", err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	answer, err := peerConnection.CreateAnswer(nil)

	if err != nil {
		return "", err
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	<-gatherComplete

	// Set connection (needed for disconnecting)
	_webrtcConnection = peerConnection

	// Tell Owncast that the Stream is now online
	_setStreamAsConnected(peerConnection)
	log.Infoln("Inbound stream connected from [NEEDS TO BE IMPLEMENTED]") // TODO: Log remote IP
	return peerConnection.LocalDescription().SDP, nil
}

func GetandValidateStreamKey(response http.ResponseWriter, request *http.Request) string {
		// Get streaming key from HTTP request header
		streamKey := ""
		authHeader := request.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			streamKey = strings.Split(authHeader, "Bearer ")[1]
		}
		
	
		if streamKey == "" {
			log.Error("Authorization was not set")
			logHTTPError(response, "Authorization was not set", http.StatusBadRequest)
		}
	
		// Check if streaming key is valid
		accessGranted := validateStreamingKey(streamKey)
		if !accessGranted {
			log.Error("Streaming Key is not valid")
			logHTTPError(response, "Authorization was not set", http.StatusBadRequest)
		}
		return streamKey
}

func whipHandler(res http.ResponseWriter, r *http.Request) {
	streamKey := GetandValidateStreamKey(res, r)
	if streamKey == "" {
		return
	}
	// Handle DELETE requests (OBS sends one when you stop streaming)
	if r.Method == "DELETE" {
		Disconnect()
		res.WriteHeader(http.StatusOK)
		return
	}

	if _hasInboundWebRTCConnection {
		log.Errorln("stream already running; can not overtake an existing stream from [NEEDS TO BE IMPLMENTED]") // TODO: Log IP from the connection
		logHTTPError(res, "stream already running; can not overtake an existing stream", http.StatusBadRequest)
		return
	}

	// Read WebRTC offer from HTTP Request
	offer, err := io.ReadAll(r.Body)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	// Create WHIP Endpoint configuration
	answer, err := WHIP(string(offer), streamKey)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}
	_hasInboundWebRTCConnection = true

	// Set 'Created' and 'Location' HTTP header to response
	res.Header().Add("Location", "/api/whip")
	res.WriteHeader(http.StatusCreated)
	fmt.Fprint(res, answer)
}

type StreamStatus struct {
	StreamKey string `json:"streamKey"`
}

func statusHandler(res http.ResponseWriter, req *http.Request) {
	statuses := []StreamStatus{}
	for _, s := range GetAllStreams() {
		statuses = append(statuses, StreamStatus{StreamKey: s})
	}

	if err := json.NewEncoder(res).Encode(statuses); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
	}
}

// Start starts the webrtc service, listening on specified RTMP port.
func Start(setStreamAsConnected func(*webrtc.PeerConnection), setBroadcaster func(models.Broadcaster)) {
	_setStreamAsConnected = setStreamAsConnected
	_setBroadcaster = setBroadcaster

	// Get WebRTC Port Number from config
	port := data.GetWebRTCPortNumber()

	// Configure WebRTC API that we will provide
	Configure()

	mux := http.NewServeMux()
	// mux.Handle("/", indexHTMLWhenNotFound(http.Dir("./web/build"))) // not needed
	mux.HandleFunc("/api/whip", corsHandler(whipHandler))
	mux.HandleFunc("/api/whep", corsHandler(whepHandler))
	mux.HandleFunc("/api/status", corsHandler(statusHandler))
	mux.HandleFunc("/api/sse/", corsHandler(whepServerSentEventsHandler))
	mux.HandleFunc("/api/layer/", corsHandler(whepLayerHandler))

	log.Println("Running WebRTC Server at `" + fmt.Sprint(port) + "`")

	log.Fatal((&http.Server{
		Handler: mux,
		Addr:    "localhost:" + fmt.Sprint(port), // TODO: get correct IP
	}).ListenAndServe())
	log.Tracef("WebRTC server is listening for incoming stream on port: %d", port)
	log.Println("Running WebRTC Server at " + fmt.Sprint(port))

	// DONE! --> We are doing this above
	// s := rtmp.NewServer()
	// var lis net.Listener
	// var err error
	// if lis, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err != nil {
	// 	log.Fatal(err)
	// }

	// TODO: We have to get the local and remote adresses from the webrtc connection
	// s.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
	// 	es := rtmp.EventString[e]
	// 	log.Traceln("RTMP", nc.LocalAddr(), nc.RemoteAddr(), es)
	// }

	// s.HandleConn = HandleConn

	// 	if err != nil {
	// 		log.Panicln(err)
	// 	}
	// 	log.Tracef("RTMP server is listening for incoming stream on port: %d", port)

	//	for {
	//		nc, err := lis.Accept()
	//		if err != nil {
	//			time.Sleep(time.Second)
	//			continue
	//		}
	//		go s.HandleNetConn(nc)
	//	}
}

// HandleConn is fired when an inbound RTMP connection takes place.
// func HandleConn(c *rtmp.Conn, nc net.Conn) {
	// TODO: Log Tags (Metadata) of Stream (will probably happen in WHIP())
	// c.LogTagEvent = func(isRead bool, t flvio.Tag) {
	// 	if t.Type == flvio.TAG_AMF0 {
	// 		log.Tracef("%+v\n", t.DebugFields())
			//DONE! --> Happens in WHIP
			//setCurrentBroadcasterInfo(t)
	// 	}
	// }
	// DONE! --> Happens in WHIP()
	// if _hasInboundWebRTCConnection {
	// 	log.Errorln("stream already running; can not overtake an existing stream from", nc.RemoteAddr().String())
	// 	_ = nc.Close()
	// 	return
	// }

	// DONE! --> Happens in whipHandler()
	// accessGranted := false
	// validStreamingKeys := data.GetStreamKeys()

	// // If a stream key override was specified then use that instead.
	// if config.TemporaryStreamKey != "" {
	// 	validStreamingKeys = []models.StreamKey{{Key: config.TemporaryStreamKey}}
	// }

	// for _, key := range validStreamingKeys {
	// 	if secretMatch(key.Key, c.URL.Path) {
	// 		accessGranted = true
	// 		break
	// 	}
	// }

	// if !accessGranted {
	// 	log.Errorln("invalid streaming key; rejecting incoming stream from", nc.RemoteAddr().String())
	// 	_ = nc.Close()
	// 	return
	// }

	// DONE! --> A log statement in WHIP()
	// rtmpOut, rtmpIn := io.Pipe()
	// _pipe = rtmpIn
	// log.Infoln("Inbound stream connected from", nc.RemoteAddr().String())
	// _setStreamAsConnected(rtmpOut)

	// // _hasInboundWebRTCConnection = true
	// _rtmpConnection = nc

	// DONE? --> I think we handle all this already
	// w := flv.NewMuxer(rtmpIn)

	// for {
	// 	if !_hasInboundWebRTCConnection {
	// 		break
	// 	}

	// 	// If we don't get a readable packet in 10 seconds give up and disconnect
	// 	if err := _rtmpConnection.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
	// 		log.Debugln(err)
	// 	}

	// 	pkt, err := c.ReadPacket()

	// 	// Broadcaster disconnected
	// 	if err == io.EOF {
	// 		handleDisconnect(nc)
	// 		return
	// 	}

	// 	// Read timeout.  Disconnect.
	// 	if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
	// 		log.Debugln("Timeout reading the inbound stream from the broadcaster.  Assuming that they disconnected and ending the stream.")
	// 		handleDisconnect(nc)
	// 		return
	// 	}

	// 	if err := w.WritePacket(pkt); err != nil {
	// 		log.Errorln("unable to write rtmp packet", err)
	// 		handleDisconnect(nc)
	// 		return
	// 	}
	// }
// }

func handleDisconnect(conn *webrtc.PeerConnection) {
	if !_hasInboundWebRTCConnection {
		return
	}
	
	// Close the WebRTC Peer Connection
	conn.Close()
	log.Infoln("Inbound stream disconnected.")
	// Delete Stream from owncast
	DeleteStream()
	_hasInboundWebRTCConnection = false
}

// Disconnect will force disconnect the current inbound WebRTC connection.
func Disconnect() {
	if _webrtcConnection == nil {
		return
	}

	log.Traceln("Inbound stream disconnect requested.")
	handleDisconnect(_webrtcConnection)
}
