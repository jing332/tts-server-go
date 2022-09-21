package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/jing332/tts-server-go/service/azure"
	"github.com/jing332/tts-server-go/service/edge"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GracefulServer struct {
	Server       *http.Server
	serveMux     *http.ServeMux
	shutdownLoad chan struct{}

	edgeLock  sync.Mutex
	azureLock sync.Mutex
}

// HandleFunc 注册
func (s *GracefulServer) HandleFunc() {
	if s.serveMux == nil {
		s.serveMux = &http.ServeMux{}
	}
	s.serveMux.Handle("/api/azure", http.TimeoutHandler(http.HandlerFunc(s.azureAPIHandler), 60*time.Second, ""))
	s.serveMux.HandleFunc("/", s.webAPIHandler)
	//s.serveMux.HandleFunc("/api/azure", s.azureAPIHandler)
	s.serveMux.HandleFunc("/api/ra", s.edgeAPIHandler)
	s.serveMux.HandleFunc("/api/legado", s.legadoAPIHandler)
}

// ListenAndServe 监听服务
func (s *GracefulServer) ListenAndServe(port int64) error {
	if s.shutdownLoad == nil {
		s.shutdownLoad = make(chan struct{})
	}

	s.Server = &http.Server{
		Addr:           ":" + strconv.FormatInt(port, 10),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler:        s.serveMux,
	}
	log.Infoln("服务已启动, 监听端口为", s.Server.Addr)
	err := s.Server.ListenAndServe()
	if err == http.ErrServerClosed { /*说明调用Shutdown关闭*/
		err = nil
	} else if err != nil {
		return err
	}

	<-s.shutdownLoad /*等待,直到服务关闭*/

	return nil
}

// Shutdown 关闭监听服务，需等待响应
func (s *GracefulServer) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := s.Server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutdown失败: %d", err)
	}

	close(s.shutdownLoad)
	s.shutdownLoad = nil

	return nil
}

//go:embed public/index.html
var indexHtml string

//go:embed public/azure.html
var azureHtml string

func (s *GracefulServer) webAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/azure.html" {
		w.Write([]byte(azureHtml))
	} else if r.URL.Path == "/index.html" || r.URL.Path == "/" {
		w.Write([]byte(indexHtml))
	} else {
		w.WriteHeader(http.StatusNotFound)
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
func (s *GracefulServer) edgeAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.edgeLock.Lock()
	defer s.edgeLock.Unlock()
	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)
	format := r.Header.Get("Format")
	b, err := edge.GetAudio(string(ssml), format)
	if err != nil {
		if e := sendErrorMsg(w, err.Error()); e != nil {
			log.Warnln("发送错误消息失败:", e)
		}
		return
	}
	w.Header().Set("Content-Type", formatContentType(format))
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=5")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))

	_, err = w.Write(b)
	if err != nil {
		log.Println("写入音频数据失败:", err)
		return
	}
}

var ttsAzure *azure.TTS

// 微软Azure TTS接口
func (s *GracefulServer) azureAPIHandler(w http.ResponseWriter, r *http.Request) {
	s.azureLock.Lock()
	defer s.azureLock.Unlock()
	format := r.Header.Get("Format")

	defer r.Body.Close()
	ssml, _ := io.ReadAll(r.Body)
	log.Infoln("接收到SSML: ", string(ssml))

	if ttsAzure == nil {
		ttsAzure = &azure.TTS{}
	}

	var succeed = make(chan []byte)
	var failed = make(chan []byte)
	go func() {
		b, err := ttsAzure.GetAudio(string(ssml), format)
		if err != nil {
			failed <- []byte(err.Error())
			return
		}
		succeed <- b
	}()

	startTime := time.Now()
	select { /* 阻塞 等待结果 */
	case b := <-succeed: /* 成功接收到音频 */
		log.Infoln("音频下载完成")
		w.Header().Set("Content-Type", formatContentType(format))
		w.Header().Set("Content-Length", strconv.FormatInt(int64(len(b)), 10))
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Keep-Alive", "timeout=5")
		_, err := w.Write(b)
		if err != nil {
			log.Warnln(err)
		}
	case b := <-failed: /* 失败 */
		log.Warnln("获取音频失败:", string(b))
		ttsAzure.CloseConn()
		ttsAzure = nil
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(b)
	case <-r.Context().Done(): /* 与阅读APP断开连接 */
		log.Warnln("客户端(阅读APP)连接 主动关闭/意外断开")
		select { /* 三秒内如果成功下载, 就保留与微软服务器的连接 */
		case <-succeed:
			log.Debugln("断开后3s内成功下载")
		case <-time.After(time.Second * 3): /* 抛弃WebSocket连接 */
			ttsAzure.CloseConn()
			ttsAzure = nil
		}
	}
	elapsedTime := time.Since(startTime) / time.Millisecond
	log.Infof("耗时: %dms\n", elapsedTime)
}

func (s *GracefulServer) legadoAPIHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	apiUrl := params.Get("api")
	name := params.Get("name")
	voiceName := params.Get("voiceName")     /* 发音人 */
	styleName := params.Get("styleName")     /* 风格 */
	styleDegree := params.Get("styleDegree") /* 风格强度(0.1-2.0) */
	roleName := params.Get("roleName")       /* 角色(身份) */
	voiceFormat := params.Get("voiceFormat") /*音频格式*/

	jsonStr, err := genLegodoJson(apiUrl, name, voiceName, styleName, styleDegree, roleName, voiceFormat)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
	}

	w.Write(jsonStr)
}

func genLegodoJson(api, name, voiceName, styleName, styleDegree, roleName, voiceFormat string) ([]byte, error) {
	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	var url string
	if styleName == "" { /* Edge大声朗读 */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}}</prosody></voice></speak>"}`
	} else { /* Azure TTS */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><mstts:express-as style=\"` + styleName + `\" styledegree=\"` + styleDegree + `\" role=\"` + roleName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;')}}</prosody> </mstts:express-as></voice></speak>"}`
	}

	head := `{"Content-Type":"text/plain","Authorization":"Bearer ","Format":"` + voiceFormat + `"}`
	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(voiceFormat), Header: head}

	body, err := json.Marshal(legadoJson)
	if err != nil {
		return nil, err
	}

	return body, nil
}

/* 根据音频格式返回对应的Content-Type */
func formatContentType(format string) string {
	t := strings.Split(format, "-")[0]
	switch t {
	case "audio":
		return "audio/mpeg"
	case "webm":
		return "audio/webm; codec=opus"
	case "ogg":
		return "audio/ogg; codecs=opus; rate=16000"
	case "riff":
		return "audio/x-wav"
	case "raw":
		if strings.HasSuffix(format, "truesilk") {
			return "audio/SILK"
		} else {
			return "audio/basic"
		}
	}
	return ""
}

type LegadoJson struct {
	ContentType    string `json:"contentType"`
	Header         string `json:"header"`
	ID             int64  `json:"id"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	//ConcurrentRate   string `json:"concurrentRate"`
	//EnabledCookieJar bool   `json:"enabledCookieJar"`
	//LoginCheckJs   string `json:"loginCheckJs"`
	//LoginUI        string `json:"loginUi"`
	//LoginURL       string `json:"loginUrl"`
}
