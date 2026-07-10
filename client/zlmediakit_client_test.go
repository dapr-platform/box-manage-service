package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

func TestAddStreamProxySendsPostParamsInQuery(t *testing.T) {
	var gotMethod string
	var gotQuery url.Values
	var gotBody map[string]interface{}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotQuery = r.URL.Query()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"key":"proxy-key"}}`))
	})}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()

	addr := listener.Addr().(*net.TCPAddr)
	client := NewZLMediaKitClient(addr.IP.String(), addr.Port, "test-secret")
	resp, err := client.AddStreamProxy(context.Background(), &AddStreamProxyRequest{
		Vhost:      "__defaultVhost__",
		App:        "live",
		Stream:     "stream_32",
		URL:        "rtsp://192.168.112.177:3012/record/videos/converted/converted_1_a4bb9542.mp4",
		RetryCount: -1,
		RtpType:    0,
		Timeout:    30,
	})
	if err != nil {
		t.Fatalf("AddStreamProxy returned error: %v", err)
	}
	if resp.Data.Key != "proxy-key" {
		t.Fatalf("unexpected proxy key: %q", resp.Data.Key)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if len(gotBody) != 0 {
		t.Fatalf("expected empty JSON body, got %#v", gotBody)
	}

	want := map[string]string{
		"secret":      "test-secret",
		"vhost":       "__defaultVhost__",
		"app":         "live",
		"stream":      "stream_32",
		"url":         "rtsp://192.168.112.177:3012/record/videos/converted/converted_1_a4bb9542.mp4",
		"retry_count": strconv.Itoa(-1),
		"rtp_type":    strconv.Itoa(0),
		"timeout_sec": strconv.Itoa(30),
	}
	for key, expected := range want {
		if actual := gotQuery.Get(key); actual != expected {
			t.Fatalf("query %s = %q, want %q; full query: %s", key, actual, expected, gotQuery.Encode())
		}
	}

	if gotPath := gotQuery.Encode(); gotPath == "" {
		t.Fatal(fmt.Errorf("expected non-empty query"))
	}
}
