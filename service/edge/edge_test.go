package edge

import (
	log "github.com/sirupsen/logrus"
	"os"
	"testing"
)

func TestEdgeApi(t *testing.T) {
	ssml := `<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"><voice name="zh-CN-XiaoxiaoNeural"><prosody rate="200%" pitch="+0Hz">　　半年后一天，苏浩再次尝试控制身上的血气运动，原本以为会一如既往般毫无动静，没想到意识操控的那部分血气竟然往控制方向移动了一丝。就是这一丝移动，让苏浩欣喜若狂。</prosody></voice></speak>`
	tts := &TTS{}
	tts.NewConn()
	audioData, err := tts.GetAudio(ssml, "webm-24khz-16bit-mono-opus")
	if err != nil {
		log.Fatal(err)
		return
	}

	os.WriteFile("webm-24khz-16bit.mp3", audioData, 0666)
}

//func TestEdgeApiRetry(t *testing.T) {
//	ssml := `错误ssml<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"><voice name="zh-CN-XiaoxiaoNeural"><prosody rate="200%" pitch="+0Hz">　　半年后一天，苏浩再次尝试控制身上的血气运动，原本以为会一如既往般毫无动静，没想到意识操控的那部分血气竟然往控制方向移动了一丝。就是这一丝移动，让苏浩欣喜若狂。</prosody></voice></speak>`
//	_, err := GetAudioForRetry(ssml, "webm-24khz-16bit-mono-opus", 3)
//	if err != nil {
//		log.Fatal(err)
//		return
//	}
//}

//func TestCloseConn(t *testing.T) {
//	go func() {
//		time.Sleep(time.Second * 3)
//		CloseConn()
//	}()
//	ssml := `<speak xmlns="http://www.w3.org/2001/10/synthesis" xmlns:mstts="http://www.w3.org/2001/mstts" xmlns:emo="http://www.w3.org/2009/10/emotionml" version="1.0" xml:lang="en-US"><voice name="zh-CN-XiaoxiaoNeural"><prosody rate="200%" pitch="+0Hz">　　半年后一天，苏浩再次尝试控制身上的血气运动，原本以为会一如既往般毫无动静，没想到意识操控的那部分血气竟然往控制方向移动了一丝。就是这一丝移动，让苏浩欣喜若狂。</prosody></voice></speak>`
//	for i := 0; i < 3; i++ {
//		_, err := GetAudio(ssml, "webm-24khz-16bit-mono-opus")
//		if err != nil {
//			t.Fatal(err)
//		}
//	}
//
//}
