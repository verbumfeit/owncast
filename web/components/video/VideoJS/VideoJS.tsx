import React, { FC, useEffect } from 'react';
import videojs from 'video.js';
import type VideoJsPlayer from 'video.js/dist/types/player';

import styles from './VideoJS.module.scss';
import { parseLinkHeader } from '@web3-storage/parse-link-header';

require('video.js/dist/video-js.css');

export type VideoJSProps = {
  options: any;
  onReady: (player: VideoJsPlayer, vjsInstance: typeof videojs) => void;
};

export const VideoJS: FC<VideoJSProps> = ({ options, onReady }) => {
  const videoRef = React.useRef<HTMLVideoElement | null>(null);
  const playerRef = React.useRef<VideoJsPlayer | null>(null);

  const [mediaSrcObject, setMediaSrcObject] = React.useState(null);
  const [videoLayers, setVideoLayers] = React.useState([]);
  const [layerEndpoint, setLayerEndpoint] = React.useState('');


  useEffect(() => {
    if (videoRef.current) {
      console.log("Media Source is:" )
      console.log(mediaSrcObject)
      videoRef.current.srcObject = mediaSrcObject
    }
  }, [mediaSrcObject, videoRef])
    
  // Establish WebRTC Peer Connection
  useEffect(() => {
    const peerConnection = new RTCPeerConnection() // eslint-disable-line

    peerConnection.ontrack = function (event) {
      console.log('streams', event.streams)
      setMediaSrcObject(event.streams[0])
      console.log(mediaSrcObject)
    }

    peerConnection.addTransceiver('audio', { direction: 'recvonly' })
    peerConnection.addTransceiver('video', { direction: 'recvonly' })

    peerConnection.createOffer().then(offer => {
      peerConnection.setLocalDescription(offer)

      fetch(`http://localhost:8090/api/whep`, { // TODO: Replace with current IP
        method: 'POST',
        body: offer.sdp,
        headers: {
          Authorization: `Bearer abc123`,
          'Content-Type': 'application/sdp'
        }
      }).then(r => {
        const parsedLinkHeader = parseLinkHeader(r.headers.get('Link'))
        setLayerEndpoint(`${window.location.protocol}//${parsedLinkHeader['urn:ietf:params:whep:ext:core:layer'].url}`)

        const evtSource = new EventSource(`${window.location.protocol}//${parsedLinkHeader['urn:ietf:params:whep:ext:core:server-sent-events'].url}`)
        evtSource.onerror = err => {
          evtSource.close();
        }

        evtSource.addEventListener("layers", event => {
          const parsed = JSON.parse(event.data)
          setVideoLayers(parsed['1']['layers'].map(l => l.encodingId))
        })

        return r.text()
      }).then(answer => {
        peerConnection.setRemoteDescription({
          sdp: answer,
          type: 'answer'
        })
      })
    })

    return function cleanup() {
      peerConnection.close()
    }
  },[])

  React.useEffect(() => {
    // Make sure Video.js player is only initialized once
    if (videoRef.current && mediaSrcObject && !playerRef.current) {
      console.log("Init VideoJS")
      console.log(videoRef)
      const videoElement = videoRef.current;
      
      // eslint-disable-next-line no-multi-assign
      const player: VideoJsPlayer = (playerRef.current = videojs(videoElement, options, () => {
        console.debug('player is ready');
        return onReady && onReady(player, videojs);
      }));

      player.autoplay(options.autoplay);
      player.src(options.sources);

       // Add a cachebuster param to playlist URLs.
      if (
        (videojs.getPlayer(videoRef.current).tech({ IWillNotUseThisInPlugins: true }) as any)?.vhs
      ) {
        (
          videojs.getPlayer(videoRef.current).tech({ IWillNotUseThisInPlugins: true }) as any
        ).vhs.xhr.beforeRequest = o => {
          if (o.uri.match('m3u8')) {
            const cachebuster = Math.random().toString(16).substr(2, 8);
            // eslint-disable-next-line no-param-reassign
            o.uri = `${o.uri}?cachebust=${cachebuster}`;
          }

          return o;
        };
      }
    }
  }, [options, videoRef, mediaSrcObject]);

  return (
    <div data-vjs-player>
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={videoRef}
        className={`video-js vjs-big-play-centered vjs-show-big-play-button-on-pause ${styles.player} vjs-owncast`}
      />
    </div>
  );
};
