package webrtcc

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/owncast/owncast/config"
	"github.com/owncast/owncast/core/data"
	"github.com/owncast/owncast/models"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
	log "github.com/sirupsen/logrus"
)

const unknownString = "Unknown"

var _streamKey = ""

var _getInboundDetailsFromMetadataRE = regexp.MustCompile(`\{(.*?)\}`)

// TODO: (Currently not used) Construct at least a mock metadata object
func getInboundDetailsFromMetadata(metadata []interface{}) (models.RTMPStreamMetadata, error) {
	metadataComponentsString := fmt.Sprintf("%+v", metadata)
	if !strings.Contains(metadataComponentsString, "onMetaData") {
		return models.RTMPStreamMetadata{}, errors.New("Not a onMetaData message")
	}

	submatchall := _getInboundDetailsFromMetadataRE.FindAllString(metadataComponentsString, 1)

	if len(submatchall) == 0 {
		return models.RTMPStreamMetadata{}, errors.New("unable to parse inbound metadata")
	}

	metadataJSONString := submatchall[0]
	var details models.RTMPStreamMetadata
	err := json.Unmarshal([]byte(metadataJSONString), &details)
	return details, err
}

func getAudioCodec(track *webrtc.TrackRemote) string {
	// DONE! --> See below
	// if codec == nil {
	// 	return "No audio"
	// }

	// var codecID float64
	// if assertedCodecID, ok := codec.(float64); ok {
	// 	codecID = assertedCodecID
	// } else {
	// 	return codec.(string)
	// }

	// switch codecID {
	// case flvio.SOUND_MP3:
	// 	return "MP3"
	// case flvio.SOUND_AAC:
	// 	return "AAC"
	// case flvio.SOUND_SPEEX:
	// 	return "Speex"
	// }

	// Check if mimetype contains audio
	if !strings.HasPrefix(track.Codec().RTPCodecCapability.MimeType, "audio") {
		return "No audio"
	}

	// Extract and return codec name
	codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
	fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), codecName)

	if codecName != "" {
		return codecName
	}
	return unknownString
}

func getVideoCodec(track *webrtc.TrackRemote) string {
	// if codec == nil {
	// 	return unknownString
	// }

	// var codecID float64
	// if assertedCodecID, ok := codec.(float64); ok {
	// 	codecID = assertedCodecID
	// } else {
	// 	return codec.(string)
	// }

	// switch codecID {
	// case flvio.VIDEO_H264:
	// 	return "H.264"
	// case flvio.VIDEO_H265:
	// 	return "H.265"
	// }

	// return unknownString

	// Check if mimetype contains audio
	if !strings.HasPrefix(track.Codec().RTPCodecCapability.MimeType, "video") {
		return "No video"
	}

	// Extract and return codec name
	codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
	fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), codecName)

	if codecName != "" {
		return codecName
	}
	return unknownString
}

func logHTTPError(w http.ResponseWriter, err string, code int) {
	log.Println(err)
	http.Error(w, err, code)
}

func secretMatch(configStreamKey string, streamingKey string) bool {
	// DONE! --> We already get the stream key from the requests HTTP Header and have it here
	// prefix := "/live/"

	// if !strings.HasPrefix(path, prefix) {
	// 	log.Debug("RTMP path does not start with " + prefix)
	// 	return false // We need the path to begin with $prefix
	// }

	// streamingKey := path[len(prefix):] // Remove $prefix

	matches := subtle.ConstantTimeCompare([]byte(streamingKey), []byte(configStreamKey)) == 1
	return matches
}

func Configure() {
	streamMap = map[string]*stream{}

	mediaEngine := &webrtc.MediaEngine{}
	if err := populateMediaEngine(mediaEngine); err != nil {
		panic(err)
	}

	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		log.Fatal(err)
	}

	settingEngine := webrtc.SettingEngine{}
	populateSettingEngine(&settingEngine)

	api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
		webrtc.WithSettingEngine(settingEngine),
	)
}

func corsHandler(next func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Access-Control-Allow-Origin", "*")
		res.Header().Set("Access-Control-Allow-Methods", "*")
		res.Header().Set("Access-Control-Allow-Headers", "*")
		res.Header().Set("Access-Control-Expose-Headers", "*")

		if req.Method != http.MethodOptions {
			next(res, req)
		}
	}
}

// Janto adapted this from owncasts code to fit the webrtc streamkey model. Probably needs some string operations to make it work
func validateStreamingKey(streamingKey string) bool {
	validStreamingKeys := data.GetStreamKeys()

	// If a stream key override was specified then use that instead.
	if config.TemporaryStreamKey != "" {
		validStreamingKeys = []models.StreamKey{{Key: config.TemporaryStreamKey}}
	}

	for _, key := range validStreamingKeys {
		if secretMatch(key.Key, streamingKey) {
			_streamKey = streamingKey
			return true
		}
	}
	return false
}

func addTrack(stream *stream, rid string) error {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	for i := range stream.videoTrackLabels {
		if rid == stream.videoTrackLabels[i] {
			return nil
		}
	}

	stream.videoTrackLabels = append(stream.videoTrackLabels, rid)
	return nil
}

func whepHandler(res http.ResponseWriter, req *http.Request) {
	streamKey := req.Header.Get("Authorization")
	if streamKey == "" {
		logHTTPError(res, "Authorization was not set", http.StatusBadRequest)
		return
	}

	offer, err := io.ReadAll(req.Body)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	answer, whepSessionId, err := WHEP(string(offer), streamKey)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	apiPath := req.Host + strings.TrimSuffix(req.URL.RequestURI(), "whep")
	res.Header().Add("Link", `<`+apiPath+"sse/"+whepSessionId+`>; rel="urn:ietf:params:whep:ext:core:server-sent-events"; events="layers"`)
	res.Header().Add("Link", `<`+apiPath+"layer/"+whepSessionId+`>; rel="urn:ietf:params:whep:ext:core:layer"`)
	res.Header().Add("Location", "/api/whep")
	res.WriteHeader(http.StatusCreated)
	fmt.Fprint(res, answer)
}

func whepServerSentEventsHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")

	vals := strings.Split(req.URL.RequestURI(), "/")
	whepSessionId := vals[len(vals)-1]

	layers, err := WHEPLayers(whepSessionId)
	if err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprint(res, "event: layers\n")
	fmt.Fprintf(res, "data: %s\n", string(layers))
	fmt.Fprint(res, "\n\n")
}

func whepLayerHandler(res http.ResponseWriter, req *http.Request) {
	var r whepLayerRequestJSON
	if err := json.NewDecoder(req.Body).Decode(&r); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}

	vals := strings.Split(req.URL.RequestURI(), "/")
	whepSessionId := vals[len(vals)-1]

	if err := WHEPChangeLayer(whepSessionId, r.EncodingId); err != nil {
		logHTTPError(res, err.Error(), http.StatusBadRequest)
		return
	}
}

func getStream(streamKey string) (*stream, error) {
	foundStream, ok := streamMap[streamKey]
	if !ok {
		audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
		if err != nil {
			return nil, err
		}

		foundStream = &stream{
			audioTrack:   audioTrack,
			pliChan:      make(chan any, 50),
			whepSessions: map[string]*whepSession{},
		}
		streamMap[streamKey] = foundStream
	}

	return foundStream, nil
}

func DeleteStream() {
	streamMapLock.Lock()
	defer streamMapLock.Unlock()

	delete(streamMap, _streamKey)
	_streamKey = ""
}
