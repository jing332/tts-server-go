package service

import (
	"strconv"
)

const (
	ApiEdge     = 0
	ApiAzure    = 1
	ApiCreation = 2
)

type VoiceProperty struct {
	Api       int
	VoiceName string
	VoiceId   string
	*Prosody
	*ExpressAs
}

// ElementString 转为Voice元素字符串
func (v *VoiceProperty) ElementString(text string) string {
	var element string
	if v.Api == ApiEdge {
		element = v.Prosody.ElementString(text)
	} else {
		element = v.ExpressAs.ElementString(text, v.Prosody)
	}

	return `<voice name="` + v.VoiceName + `">` + element + `</voice>`
}

type Prosody struct {
	Rate, Volume, Pitch int8
}

func (p *Prosody) ElementString(text string) string {
	return `<prosody rate="` + strconv.Itoa(int(p.Rate)) +
		`%" volume="` + strconv.Itoa(int(p.Volume)) +
		`%" pitch="` + strconv.Itoa(int(p.Pitch)) +
		`%">` + text + `</prosody>`
}

type ExpressAs struct {
	Style       string
	StyleDegree float32
	Role        string
}

func (e *ExpressAs) ElementString(text string, prosody *Prosody) string {
	return `<mstts:express-as style="` + e.Style +
		`" styledegree="` + strconv.FormatFloat(float64(e.StyleDegree), 'f', 1, 32) +
		`" role="` + e.Role +
		`">"` + prosody.ElementString(text) +
		`</mstts:express-as>`
}
