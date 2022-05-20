default:
	$(CC) -Wall -Werror -O2 -c -fPIC ../ringbuf.c ../rnnoise/src/*.c module.c
	$(CC) -o rnnoise_ladspa.so *.o -shared -Wl,--version-script=export.txt -lm
