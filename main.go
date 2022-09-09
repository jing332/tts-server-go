package main

import (
	"flag"
	logformat "github.com/antonfisher/nested-logrus-formatter"
	log "github.com/sirupsen/logrus"
	"time"
)

func GetISOTime() string {
	T := time.Now().String()
	return T[:23][:10] + "T" + T[:23][11:] + "Z"

}

var azureHost = flag.String("ah", "", "自定义域名，用来加速微软服务器")
var port = flag.Int64("port", 1233, "自定义监听端口")

func main() {
	flag.Parse()
	SetWssUrl(*azureHost)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&logformat.Formatter{HideKeys: true,
		TimestampFormat: "01-02|15:04:05",
	})
	StartServer(*port)
}
