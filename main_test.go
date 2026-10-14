package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPingにpongを返す(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)

	newRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ステータスコード: got %d, want %d", w.Code, http.StatusOK)
	}
	if got, want := w.Body.String(), `{"message":"pong"}`; got != want {
		t.Fatalf("レスポンスボディ: got %s, want %s", got, want)
	}
}

func TestListenアドレスはLambdaの指定に従う(t *testing.T) {
	t.Setenv("AWS_LAMBDA_HTTP_ENDPOINT", "127.0.0.1:9001")

	if got, want := listenAddr(), "127.0.0.1:9001"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestListenアドレスは未指定ならローカル用の既定値になる(t *testing.T) {
	t.Setenv("AWS_LAMBDA_HTTP_ENDPOINT", "")

	if got, want := listenAddr(), "localhost:3000"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestReadyはランタイムAPIに仕様どおりのPOSTを送る(t *testing.T) {
	var gotMethod, gotPath, gotContentType, gotBody string
	runtimeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotContentType, gotBody = r.Method, r.URL.Path, r.Header.Get("Content-Type"), string(b)
	}))
	defer runtimeAPI.Close()

	if err := reportReady(strings.TrimPrefix(runtimeAPI.URL, "http://")); err != nil {
		t.Fatalf("エラーを返した: %v", err)
	}

	if gotMethod != http.MethodPost || gotPath != "/2025-09-01/ready" {
		t.Errorf("リクエスト: got %s %s, want POST /2025-09-01/ready", gotMethod, gotPath)
	}
	if gotContentType != "application/json" || gotBody != "{}" {
		t.Errorf("Content-Type/ボディ: got %q %q, want \"application/json\" \"{}\"", gotContentType, gotBody)
	}
}

func TestReadyはランタイムAPIが失敗を返したらエラーになる(t *testing.T) {
	runtimeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer runtimeAPI.Close()

	if err := reportReady(strings.TrimPrefix(runtimeAPI.URL, "http://")); err == nil {
		t.Fatal("エラーを返さなかった")
	}
}

func TestReadyを通知した時点でサーバーはリクエストを受け付けられる(t *testing.T) {
	gin.SetMode(gin.TestMode)
	addr := freeAddr(t)

	// プラットフォームは ready を受け取った直後からリクエストを流してくる
	ready := make(chan struct{})
	runtimeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready)
	}))
	defer runtimeAPI.Close()

	go func() {
		if err := run(addr, strings.TrimPrefix(runtimeAPI.URL, "http://"), newRouter()); err != nil {
			t.Errorf("run がエラーを返した: %v", err)
		}
	}()

	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("ready が通知されなかった")
	}
	res, err := http.Get("http://" + addr + "/ping")
	if err != nil {
		t.Fatalf("ready 通知直後に接続できなかった: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("ready 通知直後の /ping: got %d, want %d", res.StatusCode, http.StatusOK)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}
