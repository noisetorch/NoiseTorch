dev:
	go generate
	go build
release:
	go generate
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w -extldflags "-static"' .
	upx noisetorch
	tar cvzf NoiseTorch_x64.tgz noisetorch
	rm noisetorch
