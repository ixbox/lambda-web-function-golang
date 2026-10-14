package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Node.js ランタイム（/var/runtime/index.js, nodejs.web24.v14）の ShimClient が使う API バージョン。
// 非公開の内部 API なので、ランタイム更新で変わったら ready 通知が 404 等で失敗する。
const runtimeAPIVersion = "2025-09-01"

func newRouter() *gin.Engine {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	return r
}

// index.js から execve で起動されるため、Lambda が Node.js プロセスに渡した
// 環境変数がそのまま引き継がれる。Node.js 側で中継せずに直接読む。
func listenAddr() string {
	if addr := os.Getenv("AWS_LAMBDA_HTTP_ENDPOINT"); addr != "" {
		return addr
	}
	return "localhost:3000"
}

// 本来は Node.js ランタイムが listen を確認してから送る通知。execve でランタイムごと
// 置き換わるため、送らないと Runtime.InitTimeout になる。
func reportReady(runtimeAPI string) error {
	res, err := http.Post(
		fmt.Sprintf("http://%s/%s/ready", runtimeAPI, runtimeAPIVersion),
		"application/json",
		strings.NewReader("{}"),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("ready 通知が失敗しました: HTTP %d", res.StatusCode)
	}
	return nil
}

// ready 通知の直後からリクエストが来るため、Serve より前に Listen を済ませておく。
// Listen 済みならバックログに積まれるので、Serve 開始前に来た接続も取りこぼさない。
func run(addr, runtimeAPI string, h http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if runtimeAPI != "" {
		if err := reportReady(runtimeAPI); err != nil {
			return err
		}
	}
	return http.Serve(ln, h)
}

func main() {
	if err := run(listenAddr(), os.Getenv("AWS_LAMBDA_RUNTIME_API"), newRouter()); err != nil {
		log.Fatal(err)
	}
}
