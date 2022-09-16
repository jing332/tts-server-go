package main

import (
	"flag"
	logformat "github.com/antonfisher/nested-logrus-formatter"
	"github.com/jing332/tts-server-go/service"
	"github.com/jing332/tts-server-go/service/azure"
	log "github.com/sirupsen/logrus"
)

var azureHost = flag.String("ah", "", "自定义域名，用来加速微软服务器")
var port = flag.Int64("port", 1233, "自定义监听端口")

func main() {
	flag.Parse()
	azure.SetWssUrl(*azureHost)
	log.SetFormatter(&logformat.Formatter{HideKeys: true,
		TimestampFormat: "01-02|15:04:05",
	})

	server := new(service.GracefulServer)
	server.HandleFunc()
	err := server.ListenAndServe(*port)
	if err != nil {
		log.Fatal(err)
	}
}
