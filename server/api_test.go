package server

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"testing"
	"time"
)

func TestShutdown(t *testing.T) {
	s := &GracefulServer{}
	s.HandleFunc()
	/*网页访问触发 用来测试关闭服务*/
	s.serveMux.HandleFunc("/shutdown", func(writer http.ResponseWriter, request *http.Request) {
		go func() {
			err := s.Shutdown(time.Second * 10)
			if err != nil {
				log.Warnln(err)
			}

		}()
	})

	err := s.ListenAndServe(1233)
	if err != nil {
		log.Println(err)
		return
	}
}
