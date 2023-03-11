package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwebrtc/livekit-whip-go/pkg/client"

	log "github.com/pion/ion-log"
)

var (
	url       = ""
	whipState *client.WhipState
)

func main() {
	flag.StringVar(&url, "url", "http://localhost:8080/whip/publish/live/stream1", "whip url")
	flag.Parse()

	whipState = &client.WhipState{}
	log.Warnf("start whip publish %v", url)
	whipState.Connect(url)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		<-sigc
		fmt.Println("Ctrl+C pressed")
		if whipState != nil {
			log.Warnf("stop whip connect")
			whipState.Close()
			whipState = nil
		}
		os.Exit(0)
	}()

	select {}
}
