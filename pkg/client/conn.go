package client

import (
	"bytes"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	log "github.com/pion/ion-log"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/webrtc/v3"

	// If you don't like x264, you can also use vpx by importing as below
	// "github.com/pion/mediadevices/pkg/codec/vpx" // This is required to use VP8/VP9 video encoder
	// or you can also use openh264 for alternative h264 implementation
	// "github.com/pion/mediadevices/pkg/codec/openh264"
	// or if you use a raspberry pi like, you can use mmal for using its hardware encoder
	//"github.com/pion/mediadevices/pkg/codec/mmal"
	//"github.com/pion/mediadevices/pkg/codec/opus" // This is required to use opus audio encoder
	"github.com/pion/mediadevices/pkg/codec/x264" // This is required to use h264 video encoder

	// Note: If you don't have a camera or microphone or your adapters are not supported,
	//       you can always swap your adapters with our dummy adapters below.
	// _ "github.com/pion/mediadevices/pkg/driver/videotest"
	// _ "github.com/pion/mediadevices/pkg/driver/audiotest"
	_ "github.com/pion/mediadevices/pkg/driver/camera"     // This is required to register camera adapter
	_ "github.com/pion/mediadevices/pkg/driver/microphone" // This is required to register microphone adapter
)

type WhipState struct {
	httpClient  *http.Client
	resourceUrl string
	whipCon     *WHIPConn
}

func (w *WhipState) Close() {
	req, err := http.NewRequest(http.MethodDelete, w.resourceUrl, nil)
	if err != nil {
		log.Errorf("http.NewRequest DELETE failed %v", err)
		return
	}
	_, err = w.httpClient.Do(req)
	if err != nil {
		log.Errorf("whipCon DELETE failed %v", err)
		return
	}
}

func (w *WhipState) Connect(whipServer string) error {

	log.Infof("Publish to whip server: %s", whipServer)

	// Create a new RTCPeerConnection
	x264Params, err := x264.NewParams()
	if err != nil {
		panic(err)
	}

	x264Params.BitRate = 500_000 // 500kbps

	/*
		opusParams, err := opus.NewParams()
		if err != nil {
			panic(err)
		}
	*/

	codecSelector := mediadevices.NewCodecSelector(
		mediadevices.WithVideoEncoders(&x264Params),
		//mediadevices.WithAudioEncoders(&opusParams),
	)

	s, err := mediadevices.GetUserMedia(mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			c.Width = prop.Int(640)
			c.Height = prop.Int(480)
		},
		//Audio: func(c *mediadevices.MediaTrackConstraints) {
		//},
		Codec: codecSelector,
	})
	if err != nil {
		log.Errorf("GetUserMedia: %v", err)
		return err
	}

	whipCon, err := NewWHIPConn()
	w.whipCon = whipCon
	if err != nil {
		log.Errorf("New WHIPConn failed %v", err)
		return err
	}
	/*
		_, err = whipCon.AddTrack(s.GetAudioTracks()[0])
		if err != nil {
			log.Errorf("whipConn.AddTrack (audioTrack) failed %v", err)
			return err
		}
	*/

	transceiver, err := whipCon.AddTrack(s.GetVideoTracks()[0])
	if err != nil {
		log.Errorf("whipConn.AddTrack (videoTrack) failed %v", err)
		return err
	}

	w.whipCon.HandleRtcpFb(transceiver.Sender())

	offer, err := whipCon.CreateOffer()
	if err != nil {
		log.Errorf("whipCon.CreateOffer failed %v", err)
		return err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	w.httpClient = client

	log.Infof("offer: %v", offer.SDP)

	resp, err := client.Post(whipServer, "application/sdp", bytes.NewBuffer([]byte(offer.SDP)))
	if err != nil {
		log.Errorf("whipCon POST offer/sdp failed %v", err)
		return err
	}

	resourceUrl := resp.Header.Get("Location")

	if resourceUrl == "" {
		resourceUrl = whipServer
	} else {
		if strings.HasPrefix(resourceUrl, "/") {
			r, err := url.Parse(whipServer)
			if err != nil {
				log.Errorf("parse url [%v] failed!", whipServer)
			}
			resourceUrl = r.Scheme + "://" + r.Host + resourceUrl
		}
	}

	log.Infof("whipCon resourceUrl %v", resourceUrl)
	w.resourceUrl = resourceUrl

	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	bodyString := string(bodyBytes)
	log.Infof("answer: %v", bodyString)
	whipCon.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: string(bodyString)})

	return nil
}
