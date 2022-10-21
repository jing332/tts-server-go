package server

import (
	"encoding/json"
	"strings"
	"time"
)

type LegadoJson struct {
	ContentType    string `json:"contentType"`
	Header         string `json:"header"`
	ID             int64  `json:"id"`
	LastUpdateTime int64  `json:"lastUpdateTime"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	ConcurrentRate string `json:"concurrentRate"`
	//EnabledCookieJar bool   `json:"enabledCookieJar"`
	//LoginCheckJs   string `json:"loginCheckJs"`
	//LoginUI        string `json:"loginUi"`
	//LoginURL       string `json:"loginUrl"`
}

type CreationJson struct {
	Text        string `json:"text"`
	VoiceName   string `json:"voiceName"`
	VoiceId     string `json:"voiceId"`
	Rate        string `json:"rate"`
	Style       string `json:"style"`
	StyleDegree string `json:"styleDegree"`
	Role        string `json:"role"`
	Volume      string `json:"volume"`
	Format      string `json:"format"`
}

/* 生成阅读APP朗读朗读引擎Json (Edge, Azure) */
func genLegodoJson(api, name, voiceName, styleName, styleDegree, roleName, voiceFormat, token, concurrentRate string) ([]byte, error) {
	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	var url string
	if styleName == "" { /* Edge大声朗读 */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}</prosody></voice></speak>"}`
	} else { /* Azure TTS */
		url = api + ` ,{"method":"POST","body":"<speak xmlns=\"http://www.w3.org/2001/10/synthesis\" xmlns:mstts=\"http://www.w3.org/2001/mstts\" xmlns:emo=\"http://www.w3.org/2009/10/emotionml\" version=\"1.0\" xml:lang=\"en-US\"><voice name=\"` +
			voiceName + `\"><mstts:express-as style=\"` + styleName + `\" styledegree=\"` + styleDegree + `\" role=\"` + roleName + `\"><prosody rate=\"{{(speakSpeed -10) * 2}}%\" pitch=\"+0Hz\">{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}</prosody> </mstts:express-as></voice></speak>"}`
	}

	head := `{"Content-Type":"text/plain","Format":"` + voiceFormat + `", "Token":"` + token + `"}`
	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(voiceFormat),
		Header: head, ConcurrentRate: concurrentRate}

	body, err := json.Marshal(legadoJson)
	if err != nil {
		return nil, err
	}

	return body, nil
}

/* 生成阅读APP朗读引擎Json (Creation) */
func genLegadoCreationJson(api, name, voiceName, voiceId, styleName, styleDegree, roleName, voiceFormat, token, concurrentRate string) ([]byte, error) {
	t := time.Now().UnixNano() / 1e6 //毫秒时间戳
	urlJsonStr := `{"text":"{{String(speakText).replace(/&/g, '&amp;').replace(/\"/g, '&quot;').replace(/'/g, '&apos;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/\\/g, '')}}","voiceName":"` +
		voiceName + `","voiceId":"` + voiceId + `","rate":"{{(speakSpeed -10) * 2}}%","style":"` + styleName + `","styleDegree":"` + styleDegree +
		`","role":"` + roleName + `","volume":"0%","format":"` + voiceFormat + `"}`
	url := api + `,{"method":"POST","body":` + urlJsonStr + `}`
	head := `{"Content-Type":"application/json", "Token":"` + token + `"}`

	legadoJson := &LegadoJson{Name: name, URL: url, ID: t, LastUpdateTime: t, ContentType: formatContentType(voiceFormat),
		Header: head, ConcurrentRate: concurrentRate}
	body, err := json.Marshal(legadoJson)
	return body, err
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
