dev: rnnoise
	mkdir -p bin/
	go generate
	go build -o bin/noisetorch
release: rnnoise
	mkdir -p bin/
	mkdir -p tmp/

	mkdir -p tmp/.local/share/icons/hicolor/256x256/apps/
	cp assets/icon/noisetorch.png tmp/.local/share/icons/hicolor/256x256/apps/

	mkdir -p tmp/.local/share/applications/
	cp assets/noisetorch.desktop tmp/.local/share/applications/

	mkdir -p tmp/.local/bin/
	go generate
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w -extldflags "-static"' .
	upx noisetorch
	mv noisetorch tmp/.local/bin/
	cd tmp/; \
	tar cvzf ../bin/NoiseTorch_x64.tgz .
	rm -rf tmp/
rnnoise:
	cd librnnoise_ladspa/; \
	cmake . -DBUILD_VST_PLUGIN=OFF -DBUILD_LV2_PLUGIN=OFF -DBUILD_LADSPA_PLUGIN=ON; \
	make

