package azure

import (
	"io/ioutil"
	"testing"
)

func TestAzureApi(t *testing.T) {
	ssml := `<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"> <voice name="zh-CN-XiaoxiaoNeural"> <mstts:express-as style="general" styledegree="1.0"> <prosody rate="0%" pitch="+0Hz">这是微软TTS测试文本。</prosody> </mstts:express-as> </voice> </speak>`
	tts := &TTS{}
	err := tts.NewConn()
	if err != nil {
		t.Fatal(err)
	}
	audioData, err := tts.GetAudio(ssml, "audio-24khz-160kbitrate-mono-mp3")
	if err != nil {
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	ioutil.WriteFile("24khz-160kbps.mp3", audioData, 6666)
}

func TestGetVoices(t *testing.T) {
	data, err := GetVoices()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
}

//func TestAzureApiRetry(t *testing.T) {
//	ssml := `错误<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"> <voice name="zh-CN-XiaoxiaoNeural"> <mstts:express-as style="general" styledegree="1.0"> <prosody rate="100%" pitch="+0Hz">测试文本</prosody> </mstts:express-as> </voice> </speak>`
//	_, err := GetAudioForRetry(ssml, "webm-24khz-16bit-mono-opus", 3)
//	if err != nil {
//		print(err)
//		return
//	}
//}

//func TestCloseConn(t *testing.T) {
//	go func() {
//		time.Sleep(time.Second * 3)
//		CloseConn()
//	}()
//	ssml := `<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"> <voice name="zh-CN-XiaoxiaoNeural"> <mstts:express-as style="general" styledegree="1.0"> <prosody rate="100%" pitch="+0Hz">测试文本</prosody> </mstts:express-as> </voice> </speak>`
//	for i := 0; i < 3; i++ {
//		_, err := GetAudio(ssml, "webm-24khz-16bit-mono-opus")
//		if err != nil {
//			t.Fatal(err)
//		}
//	}
//}
