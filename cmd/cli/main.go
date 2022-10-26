package main

import (
	"flag"
	logformat "github.com/antonfisher/nested-logrus-formatter"
	"github.com/jing332/tts-server-go/server"
	log "github.com/sirupsen/logrus"
)

var port = flag.Int64("port", 1233, "自定义监听端口")
var token = flag.String("token", "", "使用token验证")

func main() {
	log.SetFormatter(&logformat.Formatter{HideKeys: true,
		TimestampFormat: "01-02|15:04:05",
	})
	flag.Parse()
	if *token != "" {
		log.Info("使用Token: ", *token)
	}

	srv := &server.GracefulServer{Token: *token}
	srv.HandleFunc()
	err := srv.ListenAndServe(*port)
	if err != nil {
		log.Fatal(err)
	}
}
