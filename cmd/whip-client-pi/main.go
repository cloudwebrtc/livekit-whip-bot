package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/pion/ion-log"
)

var (
	whipURL   = ""
	whipState *WhipState
)

func showHelp() {
	fmt.Printf("Usage:%s {params}\n", os.Args[0])
	fmt.Printf("Params:\n")
	fmt.Println("      -url {whip url, e.g http://localhost:8080/whip/publish/live/stream1}")
}

func main() {
	flag.StringVar(&whipURL, "url", "", "whip url")
	flag.Parse()

	if whipURL == "" {
		showHelp()
		return
	}

	whipState = &WhipState{}
	log.Warnf("start whip publish %v", whipURL)
	whipState.Connect(whipURL)

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
