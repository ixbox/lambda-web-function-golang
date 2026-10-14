# lambda-web-function-golang

AWS Lambda Web Functions で Go（gin）の HTTP サーバーを動かす PoC です。

Lambda Web Functions が対応しているランタイムは、現時点では Node.js（`nodejs24.x`）だけです。そこでエントリポイントの `index.js` から `process.execve` を呼び、Node.js プロセスを同梱の Go バイナリに置き換えて処理を委譲します。

## 仕組み

```
Lambda (nodejs24.x ランタイム)
  └─ /var/runtime/index.js が /var/task/index.js を import
       └─ process.execve で同じ PID のまま /var/task/bootstrap (Go) に置き換わる
            ├─ AWS_LAMBDA_HTTP_ENDPOINT で listen
            └─ AWS_LAMBDA_RUNTIME_API に POST /2025-09-01/ready を送る
```

Node.js ランタイムは、ユーザーコードを import して listen を確かめたあと、Runtime API に ready を通知して init 完了を報告します。execve をするとその処理ごとプロセスが置き換わるので、同じ通知を Go 側（`main.go` の `reportReady`）で送っています。この通知がないと、listen していても `Runtime.InitTimeout` で起動に失敗します。

## ファイル構成

| ファイル | 役割 |
|---|---|
| `index.js` | エントリポイント。`process.execve` で `bootstrap` に置き換える |
| `main.go` | gin のサーバー（`GET /ping`）と ready 通知 |
| `main_test.go` | テスト |
| `Makefile` | ビルドとデプロイ |

## 必要なもの

- Go
- AWS CLI v2（`aws lambda-web` コマンドを含むバージョン）
- デプロイ先の AWS 認証情報

## 使い方

```sh
# テスト
make test

# ローカル実行（http://localhost:3000/ping）
go run .

# ビルド（dist/ に index.js と linux/arm64 の bootstrap を出力）
make build

# デプロイ（既定値は NAME=golang, REGION=ap-northeast-1）
make deploy
make deploy NAME=my-func REGION=us-east-1
```

Lambda Web Functions は Graviton（linux/arm64）で動くので、`GOOS=linux GOARCH=arm64` でクロスビルドしています。

## 検証結果

| 構成 | init | init 時のメモリ | 常駐プロセス |
|---|---|---|---|
| execve のみ | `Runtime.InitTimeout`（失敗） | 139 MB | Go |
| spawn（Node.js の子プロセスとして Go を起動） | 成功（約 320 ms） | 約 155 MB | Node.js と Go |
| execve と Go 側の ready 通知（この実装） | 成功（約 290 ms） | 138 MB | Go |

数値は ap-northeast-1、ランタイム `nodejs.web24.v14` で1回ずつ計測したもので、目安です。

## 注意点

- ready 通知の API バージョン `2025-09-01` は、ランタイム内部（`/var/runtime/index.js`）から読み取った非公開の値です。Lambda Web Functions ではランタイムのバージョンを固定できず、自動更新で変わると起動に失敗します。
- 内部 API に依存したくない場合は spawn 方式が使えます。Node.js が常駐してメモリを多く使い、SIGTERM の中継も必要になりますが、ready 通知はランタイムがそのまま行います。
- gin は debug モードのままです。SIGTERM を受けたときのグレースフルシャットダウンも実装していません。

## 参考

- [AWS Lambda Web Functions](https://aws.amazon.com/jp/lambda/lambda-web-functions/)
- [Lambda Web Functions ドキュメント](https://docs.aws.amazon.com/lambda/latest/dg/web-functions.html)
