package tts_server_go

import "testing"

func TestGetIP(t *testing.T) {
	netIp, err := GetOutboundIP()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(netIp.String())
}
