PREFIX = /usr/local

default: 
	$(CC) -Wall -Werror -O2 -c -fPIC ../ringbuf.c ../rnnoise/*.c module.c 
	$(CC) -o rnnoise_ladspa.so *.o -shared -Wl,--version-script=export.txt -lm

install: default
	install -Dm755 rnnoise_ladspa.so $(DESTDIR)$(PREFIX)/lib/ladspa/rnnoise_ladspa.so
