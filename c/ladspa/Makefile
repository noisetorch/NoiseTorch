default:
	$(CC) -I ../rnnoise/include -Wall -Werror -Wno-null-dereference -O2 -c -fPIC ../c-ringbuf/ringbuf.c ../rnnoise/src/*.c module.c
	$(CC) -o rnnoise_ladspa.so *.o -shared -Wl -lm
