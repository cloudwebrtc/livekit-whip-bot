package whip

import (
	"log"
	"net"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Candidates struct {
	IceLite    bool     `mapstructure:"icelite"`
	NAT1To1IPs []string `mapstructure:"nat1to1"`
}

// ICEServerConfig defines parameters for ice servers
type ICEServerConfig struct {
	URLs       []string `mapstructure:"urls"`
	Username   string   `mapstructure:"username"`
	Credential string   `mapstructure:"credential"`
}

// WebRTCConfig defines parameters for ice
type WebRTCConfig struct {
	ICESinglePort int               `mapstructure:"singleport"`
	ICEPortRange  []uint16          `mapstructure:"portrange"`
	ICEServers    []ICEServerConfig `mapstructure:"iceserver"`
	Candidates    Candidates        `mapstructure:"candidates"`
}

// Config for base SFU
type Config struct {
	WebRTC WebRTCConfig `mapstructure:"webrtc"`
}

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

func Init(c Config) {
	webrtcSettings = webrtc.SettingEngine{}

	if c.WebRTC.ICESinglePort != 0 {
		log.Print("Listen on ", "single-port: ", c.WebRTC.ICESinglePort)
		udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IP{0, 0, 0, 0},
			Port: c.WebRTC.ICESinglePort,
		})
		if err != nil {
			panic(err)
		}
		webrtcSettings.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udpListener))
	} else {
		var icePortStart, icePortEnd uint16

		if len(c.WebRTC.ICEPortRange) == 2 {
			icePortStart = c.WebRTC.ICEPortRange[0]
			icePortEnd = c.WebRTC.ICEPortRange[1]
		}
		if icePortStart != 0 || icePortEnd != 0 {
			if err := webrtcSettings.SetEphemeralUDPPortRange(icePortStart, icePortEnd); err != nil {
				panic(err)
			}
		}
	}

	var iceServers []webrtc.ICEServer
	if c.WebRTC.Candidates.IceLite {
		webrtcSettings.SetLite(c.WebRTC.Candidates.IceLite)
	} else {
		for _, iceServer := range c.WebRTC.ICEServers {
			s := webrtc.ICEServer{
				URLs:       iceServer.URLs,
				Username:   iceServer.Username,
				Credential: iceServer.Credential,
			}
			iceServers = append(iceServers, s)
		}
	}

	if len(c.WebRTC.Candidates.NAT1To1IPs) > 0 {
		webrtcSettings.SetNAT1To1IPs(c.WebRTC.Candidates.NAT1To1IPs, webrtc.ICECandidateTypeHost)
	}
}

type WHIPConn struct {
	pc                      *webrtc.PeerConnection
	OnTrack                 func(pc *webrtc.PeerConnection, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)
	OnConnectionStateChange func(s webrtc.PeerConnectionState)
	tracks                  []*webrtc.TrackRemote
}

func NewWHIPConn() (*WHIPConn, error) {

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mineTypePCMA, ClockRate: 8000},
			PayloadType:        8,
		},
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
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mimeTypeVP8, ClockRate: 90000, RTCPFeedback: videoRTCPFeedback},
			PayloadType:        96,
		},
		{
			RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", RTCPFeedback: videoRTCPFeedback},
			PayloadType:        102,
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
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(webrtcSettings), webrtc.WithInterceptorRegistry(i))

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

	// Accept one audio and one video track incoming
	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Print(err)
		}
	}

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().MimeType)
		whip.tracks = append(whip.tracks, track)
		if whip.OnTrack != nil {
			go whip.OnTrack(peerConnection, track, receiver)
		}
	})

	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State has changed: %s\n", s.String())
		if whip.OnConnectionStateChange != nil {
			go whip.OnConnectionStateChange(s)
		}
	})

	return whip, nil
}

func (w *WHIPConn) AddTrack(track webrtc.TrackLocal) (*webrtc.RTPSender, error) {
	return w.pc.AddTrack(track)
}

func (w *WHIPConn) Offer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	// Set the remote SessionDescription
	err := w.pc.SetRemoteDescription(offer)
	if err != nil {
		log.Printf("SetRemoteDescription err %v ", err)
		w.pc.Close()
		return nil, err
	}

	// Create an answer
	answer, err := w.pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("CreateAnswer err %v ", err)
		w.pc.Close()
		return nil, err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(w.pc)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = w.pc.SetLocalDescription(answer); err != nil {
		log.Printf("SetLocalDescription err %v ", err)
		w.pc.Close()
		return nil, err
	}

	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	return w.pc.LocalDescription(), nil
}

func (w *WHIPConn) AddICECandidate(candidate webrtc.ICECandidateInit) error {
	return w.pc.AddICECandidate(candidate)
}

func (w *WHIPConn) PictureLossIndication() {
	for _, track := range w.tracks {
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			errSend := w.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
			if errSend != nil {
				log.Println(errSend)
				return
			}
		}
	}
}

func (w *WHIPConn) Close() {
	if w.pc != nil && w.pc.ConnectionState() != webrtc.PeerConnectionStateClosed {
		if cErr := w.pc.Close(); cErr != nil {
			log.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}
}
