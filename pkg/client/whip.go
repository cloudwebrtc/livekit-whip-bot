package client

import (
	"net"

	"github.com/pion/interceptor"
	log "github.com/pion/ion-log"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

var (
	webrtcSettings webrtc.SettingEngine
)

const (
	mimeTypeH264 = "video/h264"
	mimeTypeOpus = "audio/opus"
	mimeTypeVP8  = "video/vp8"
	mimeTypeVP9  = "video/vp9"
	mineTypePCMA = "audio/PCMA"
)

func init() {
	webrtcSettings = webrtc.SettingEngine{}
	udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IP{0, 0, 0, 0},
		Port: 50160,
	})
	if err != nil {
		panic(err)
	}
	webrtcSettings.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))
}

type WHIPConn struct {
	pc      *webrtc.PeerConnection
	OnTrack func(pc *webrtc.PeerConnection, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)
}

func NewWHIPConn() (*WHIPConn, error) {

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1", RTCPFeedback: nil},
			PayloadType:        111,
		},
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeAudio); err != nil {
			return nil, err
		}
	}

	videoRTCPFeedback := []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}

	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", RTCPFeedback: videoRTCPFeedback},
			PayloadType:        125,
		},
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
			return nil, err
		}
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	i := &interceptor.Registry{}

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m) /*webrtc.WithSettingEngine(webrtcSettings),*/, webrtc.WithInterceptorRegistry(i))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers:   []webrtc.ICEServer{},
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
		//RTCPMuxPolicy: webrtc.RTCPMuxPolicyRequire,
		BundlePolicy: webrtc.BundlePolicyBalanced,
	}
	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	whip := &WHIPConn{
		pc: peerConnection,
	}
	/*
		// Accept one audio and one video track incoming
		for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
			if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
				Direction: webrtc.RTPTransceiverDirectionRecvonly,
			}); err != nil {
				log.Infof("%v", err)
			}
		}*/

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Infof("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().MimeType)

		if whip.OnTrack != nil {
			go whip.OnTrack(peerConnection, track, receiver)
		}
	})

	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Infof("Peer Connection State has changed: %s\n", s.String())
	})

	return whip, nil
}

func (w *WHIPConn) AddTrack(track webrtc.TrackLocal) (*webrtc.RTPTransceiver, error) {
	return w.pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})
}

func (w *WHIPConn) CreateOffer() (*webrtc.SessionDescription, error) {
	// Create an offer
	offer, err := w.pc.CreateOffer(nil)
	if err != nil {
		log.Infof("CreateOffer err %v ", err)
		w.pc.Close()
		return nil, err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(w.pc)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = w.pc.SetLocalDescription(offer); err != nil {
		log.Infof("SetLocalDescription err %v ", err)
		w.pc.Close()
		return nil, err
	}

	<-gatherComplete

	return w.pc.LocalDescription(), nil
}

func (w *WHIPConn) SetRemoteDescription(desc webrtc.SessionDescription) error {
	// Set the remote SessionDescription
	err := w.pc.SetRemoteDescription(desc)
	if err != nil {
		log.Infof("SetRemoteDescription err %v ", err)
		w.pc.Close()
		return err
	}
	return nil
}

func (w *WHIPConn) Answer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	// Set the remote SessionDescription
	err := w.pc.SetRemoteDescription(offer)
	if err != nil {
		log.Infof("SetRemoteDescription err %v ", err)
		w.pc.Close()
		return nil, err
	}

	// Create an answer
	answer, err := w.pc.CreateAnswer(nil)
	if err != nil {
		log.Infof("CreateAnswer err %v ", err)
		w.pc.Close()
		return nil, err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(w.pc)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = w.pc.SetLocalDescription(answer); err != nil {
		log.Infof("SetLocalDescription err %v ", err)
		w.pc.Close()
		return nil, err
	}

	<-gatherComplete

	return w.pc.LocalDescription(), nil
}

func (w *WHIPConn) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	return w.pc.AddICECandidate(candidate)
}

func (w *WHIPConn) Close() {
	if w.pc != nil && w.pc.ConnectionState() != webrtc.PeerConnectionStateClosed {
		if cErr := w.pc.Close(); cErr != nil {
			log.Infof("cannot close peerConnection: %v\n", cErr)
		}
	}
}

func (w *WHIPConn) HandleRtcpFb(rtpSender *webrtc.RTPSender) {

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			n, _, rtcpErr := rtpSender.Read(rtcpBuf)
			if rtcpErr != nil {
				return
			}
			bytes := rtcpBuf[:n]
			pkts, err := rtcp.Unmarshal(bytes)
			if err != nil {
				log.Errorf("Unmarshal rtcp receiver packets err %v", err)
			}

			var fwdPkts []rtcp.Packet
			pliOnce := true
			firOnce := true
			var (
				maxRatePacketLoss  uint8
				expectedMinBitrate uint64
			)
			for _, pkt := range pkts {
				switch p := pkt.(type) {
				case *rtcp.PictureLossIndication:
					if pliOnce {
						fwdPkts = append(fwdPkts, p)
						log.Infof("PictureLossIndication")
						//TODO: hi.CameraSendKeyFrame()
						pliOnce = false
					}
				case *rtcp.FullIntraRequest:
					if firOnce {
						fwdPkts = append(fwdPkts, p)
						//log.Infof("FullIntraRequest")
						firOnce = false
					}
				case *rtcp.ReceiverEstimatedMaximumBitrate:
					if expectedMinBitrate == 0 || expectedMinBitrate > uint64(p.Bitrate) {
						expectedMinBitrate = uint64(p.Bitrate)
						//TODO: hi.CameraUpdateBitrate(uint32(expectedMinBitrate / 1024))
						log.Infof("ReceiverEstimatedMaximumBitrate %d", expectedMinBitrate)
					}
				case *rtcp.ReceiverReport:
					for _, r := range p.Reports {
						if maxRatePacketLoss == 0 || maxRatePacketLoss < r.FractionLost {
							maxRatePacketLoss = r.FractionLost
							//log.Infof("maxRatePacketLoss %d", maxRatePacketLoss)
						}
					}
				case *rtcp.TransportLayerNack:
				}
			}
		}
	}()
}
