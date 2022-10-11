package main

import (
	"flag"
	logformat "github.com/antonfisher/nested-logrus-formatter"
	"github.com/jing332/tts-server-go/server"
	log "github.com/sirupsen/logrus"
)

var port = flag.Int64("port", 1233, "自定义监听端口")

func main() {
	flag.Parse()
	log.SetFormatter(&logformat.Formatter{HideKeys: true,
		TimestampFormat: "01-02|15:04:05",
	})

	server := new(server.GracefulServer)
	server.HandleFunc()
	err := server.ListenAndServe(*port)
	if err != nil {
		log.Fatal(err)
	}
}
