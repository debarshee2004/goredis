run: build
	@./bin/goredis --listenAddress :5555

build:
	@go build -o bin/goredis .