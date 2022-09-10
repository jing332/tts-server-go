package azure

import (
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestEdgeApi(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	ssml := `<speak xmlns="http://www.w3.org/2001/10/synthesis"
xmlns:mstts="http://www.w3.org/2001/mstts"
xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0"
xml:lang="en-US">
    <voice name="zh-CN-XiaoxiaoNeural">
        <mstts:express-as style="general" styledegree="1.0">
            <prosody rate="100%" pitch="+0Hz">测试文本</prosody>
        </mstts:express-as>
    </voice>
</speak>`
	_, err := GetAudio(ssml, "webm-24khz-16bit-mono-opus")
	if err != nil {
		print(err)
		return
	}
}
