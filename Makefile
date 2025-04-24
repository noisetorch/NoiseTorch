NAME_SUFFIX=
UPDATE_URL=https://github.com/noisetorch/NoiseTorch/releases/download/
WEBSITE_URL=https://github.com/noisetorch/NoiseTorch

UPDATE_PUBKEY=Md2rdsS+b6W0trgcqa5lAWP978Zj0sFmubJ252OPKwc=
VERSION := $(shell git describe --tags)

dev: rnnoise
	mkdir -p bin/
	go generate
	go build -ldflags '-X main.nameSuffix=${NAME_SUFFIX}_(dev) -X main.version=${VERSION} -X main.websiteURL=${WEBSITE_URL}' -o bin/noisetorch
release_build: rnnoise
	mkdir -p bin/
	go generate
	CGO_ENABLED=0 GOOS=linux go build -trimpath -tags release -a -ldflags '-s -w -extldflags "-static" -X main.nameSuffix=${NAME_SUFFIX} -X main.version=${VERSION} -X main.distribution=official -X main.updateURL=${UPDATE_URL} -X main.publicKeyString=${UPDATE_PUBKEY} -X main.websiteURL=${WEBSITE_URL}' .
release: rnnoise release_build
	mkdir -p tmp/

	mkdir -p tmp/.local/share/icons/hicolor/256x256/apps/
	cp assets/icon/noisetorch.png tmp/.local/share/icons/hicolor/256x256/apps/

	mkdir -p tmp/.local/share/applications/
	cp assets/noisetorch.desktop tmp/.local/share/applications/

	mkdir -p tmp/.local/bin/
	cp noisetorch tmp/.local/bin/
	cd tmp/; \
	tar cvzf ../bin/NoiseTorch_x64_${VERSION}.tgz .
	rm -rf tmp/
	go run scripts/signer.go -s -f bin/NoiseTorch_x64_${VERSION}.tgz
rnnoise:
	git submodule update --init --recursive
	$(MAKE) -C c/ladspa
install: rnnoise release_build
	mkdir -p ~/.local/share/icons/hicolor/256x256/apps/
	mkdir -p ~/.local/share/applications/
	mkdir -p ~/.local/bin/
	cp noisetorch ~/.local/bin/
	cp assets/noisetorch.desktop ~/.local/share/applications/
	cp assets/icon/noisetorch.png ~/.local/share/icons/hicolor/256x256/apps/
uninstall:
	rm ~/.local/share/icons/hicolor/256x256/apps/noisetorch.png || true
	rm ~/.local/share/applications/noisetorch.desktop || true
	rm ~/.local/bin/noisetorch || true

