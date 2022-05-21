UPDATE_URL=
UPDATE_PUBKEY=
VERSION := $(shell git describe --tags)

dev: rnnoise
	mkdir -p bin/
	go generate
	go build -ldflags '-X main.version=${VERSION}' -o bin/noisetorch
release: rnnoise
	mkdir -p bin/
	mkdir -p tmp/

	mkdir -p tmp/.local/share/icons/hicolor/256x256/apps/
	cp assets/icon/noisetorch.png tmp/.local/share/icons/hicolor/256x256/apps/

	mkdir -p tmp/.local/share/applications/
	cp assets/noisetorch.desktop tmp/.local/share/applications/

	mkdir -p tmp/.local/bin/
	go generate
	CGO_ENABLED=0 GOOS=linux go build -trimpath -tags release -a -ldflags '-s -w -extldflags "-static" -X main.version=${VERSION} -X main.distribution=official' .
	upx noisetorch
	mv noisetorch tmp/.local/bin/
	cd tmp/; \
	tar cvzf ../bin/NoiseTorch_x64.tgz .
	rm -rf tmp/
	go run scripts/signer.go -s
	git describe --tags > bin/version.txt
rnnoise:
	$(MAKE) -C c/ladspa
