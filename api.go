package main

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
	"tts-server-go/service/azure"
)

var rwLock sync.RWMutex

func StartServer(port int64) {
	http.HandleFunc("/api/azure", azureAPIHandler)
	s := &http.Server{
		Addr:           ":" + strconv.FormatInt(port, 10),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Infoln("服务已启动, 监听端口为", s.Addr)
	err := s.ListenAndServe()
	if err != nil {
		log.Fatal("listenAndServe: ", err)
	}
}

// 微软Azure接口
func azureAPIHandler(w http.ResponseWriter, r *http.Request) {
	rwLock.Lock()
	defer rwLock.Unlock()
	format := `webm-24khz-16bit-mono-opus`
	ctxType := `audio/webm; codec=opus`

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Infoln(err)
		}
	}(r.Body)

	ssml, _ := io.ReadAll(r.Body)

	b, err := azure.GetAudio(string(ssml), format)
	if err != nil {
		log.Println("获取音频失败:", err)
		return
	}
	w.Header().Set("Content-Type", ctxType)
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Println("写入数据失败:", err)
		return
	}
}
