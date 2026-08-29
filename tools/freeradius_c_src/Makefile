
CC = gcc
CFLAGS = -fPIC -Wall -I. -I/usr/include/freeradius -I/usr/include
LDFLAGS = -shared -llmdb

TARGET = rlm_lmdb.so
SRC = rlm_lmdb.c

all: $(TARGET)

$(TARGET): $(SRC)
	$(CC) $(CFLAGS) $(SRC) $(LDFLAGS) -o $(TARGET)

clean:
	rm -f $(TARGET)
