package client

import "testing"

func TestLooksLikeSmartVisionTextResponse(t *testing.T) {
	if !looksLikeSmartVisionTextResponse([]byte("SmartVision download err"), "") {
		t.Fatal("expected printable tiny response to be treated as text")
	}
	if !looksLikeSmartVisionTextResponse([]byte{0x00, 0x01, 0x02}, "application/json") {
		t.Fatal("expected JSON content type to be treated as text response")
	}
	if looksLikeSmartVisionTextResponse([]byte{0x00, 0x01, 0x02}, "application/octet-stream") {
		t.Fatal("expected binary octet-stream preview to be treated as file content")
	}
}
