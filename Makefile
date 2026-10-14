NAME   ?= golang
REGION ?= ap-northeast-1
DIST   := dist

.PHONY: build deploy test clean

build:
	mkdir -p $(DIST)
	# Lambda Web Functions は Graviton (linux/arm64) で動作する
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o $(DIST)/bootstrap .
	cp index.js $(DIST)/

deploy: build
	aws lambda-web deploy --name $(NAME) --code $(DIST)/ --create --region $(REGION)

test:
	go test ./...

clean:
	rm -rf $(DIST)
