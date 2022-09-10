package main

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
	"tts-server-go/service/azure"
	"tts-server-go/service/edge"
)

var rwLock sync.RWMutex

func StartServer(port int64) {
	http.HandleFunc("/api/azure", azureAPIHandler)
	http.HandleFunc("/api/ra", edgeAPIHandler)
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

func sendErrorMsg(w http.ResponseWriter, msg string) error {
	log.Warnln("获取音频失败:", msg)
	w.WriteHeader(http.StatusServiceUnavailable)

	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return nil
}

// Microsoft Edge 大声朗读接口
func edgeAPIHandler(w http.ResponseWriter, r *http.Request) {
	rwLock.Lock()
	defer rwLock.Unlock()
	format := `webm-24khz-16bit-mono-opus`
	ctxType := `audio/webm; codec=opus`

	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)

	b, err := edge.GetAudio(string(ssml), format)
	if err != nil {
		if e := sendErrorMsg(w, err.Error()); e != nil {
			log.Warnln("发送错误消息失败:", e)
		}
		return
	}
	w.Header().Set("Content-Type", ctxType)
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Println("写入音频数据失败:", err)
		return
	}
}

// 微软Azure TTS接口
func azureAPIHandler(w http.ResponseWriter, r *http.Request) {
	rwLock.Lock()
	defer rwLock.Unlock()
	format := `webm-24khz-16bit-mono-opus`
	ctxType := `audio/webm; codec=opus`

	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)

	b, err := azure.GetAudio(string(ssml), format)
	if err != nil {
		if e := sendErrorMsg(w, err.Error()); e != nil {
			log.Warnln("发送错误消息失败:", e)
		}
		return
	}

	w.Header().Set("Content-Type", ctxType)
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Warnln("写入音频数据失败:", err)
		return
	}
}
