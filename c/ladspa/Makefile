default: 
	$(CC) -Wall -O2 -c -fPIC ../ringbuf.c ../rnnoise/*.c module.c
	$(CC) -shared -lm -Wl,--version-script=export.txt -o rnnoise_ladspa.so *.o
