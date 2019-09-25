#ifndef _IO_H
#define _IO_H

#include <stdio.h>
#include <stdlib.h>
#include "MatroskaParser.h"

typedef struct IO {
    InputStream input;
    longlong pos;
    FILE *file;
} IO;

void set_callbacks(IO *input);
unsigned int get_width(TrackInfo *info);
unsigned int get_height(TrackInfo *info);

IO* open_io(char *file);
int free_io(IO *io);

#endif
