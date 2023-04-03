# livekit-whip-bot

A WHIP tool library for pushing video streams from embedded boards to livekit-server.

## Running the examples

```bash
git clone https://github.com/cloudwebrtc/livekit-whip-bot
cd livekit-whip-bot
```

### Run WHIP Server

Modify the config.toml file,
Replace it with your own livekit server and API key/secret

```toml
[livekit]
server = 'http://localhost:7880'
api_key = ""
api_secret = ""
```

```bash
# Run server
go run cmd/one2many/main.go -c config.toml
```

### Run livekit WHIP bot

Install the golang development environment on your Raspberry Pi 3B/4B or zero, and clone this repository to your Raspberry Pi linux system.

```bash
# ssh pi@raspberrypi.local
git clone https://github.com/cloudwebrtc/livekit-whip-bot
cd livekit-whip-bot && go mod tidy
go build -o livekit-whip-bot cmd/whip-client-pi/*.go
```

then publish the whip stream and you should be able to see your pi 📸️ in the livekit room

```bash
./livekit-whip-bot --url http://192.168.1.141:8080/whip/publish/live/my-pi-cam
```

Note: Please replace `live` with the actual room name of your livekit server, replace `192.168.1.141:8080` with the IP:port of your WHIP server


### Screenshots

<img width="500" height="348" src="https://raw.githubusercontent.com/cloudwebrtc/livekit-whip-bot/main/screenshots/livekit-whp-bot.jpg"/>
<img width="500" height="348" src="https://raw.githubusercontent.com/cloudwebrtc/livekit-whip-bot/main/screenshots/pi-zero-2w.jpg"/>
