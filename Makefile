dev: rnnoise
	mkdir -p bin/
	go generate
	go build -o bin/noisetorch
release: rnnoise
	mkdir -p bin/
	go generate
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-s -w -extldflags "-static"' .
	upx noisetorch
	tar cvzf NoiseTorch_x64.tgz noisetorch
	rm noisetorch
	mv NoiseTorch_x64.tgz bin/
rnnoise:
	cd librnnoise_ladspa/; \
	cmake . -DBUILD_VST_PLUGIN=OFF -DBUILD_LV2_PLUGIN=OFF -DBUILD_LADSPA_PLUGIN=ON; \
	make

