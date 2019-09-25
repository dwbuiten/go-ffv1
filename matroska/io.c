#include <stdlib.h>
#include <string.h>

#include "MatroskaParser.h"
#include "io.h"

static int ioread(struct IO *cc, ulonglong pos, void *buffer, int count)
{
    if (count == 0)
        return 0;

    if (pos != cc->pos) {
        int ret = fseeko(cc->file, pos, SEEK_SET);
        if (ret < 0)
            return -1;
        cc->pos = pos;
    }

    int ret = fread(buffer, 1, count, cc->file);
    if (ret < 0)
        return -1;
    cc->pos += (ulonglong) count;

    return ret;
}

static longlong scan(struct IO *cc, ulonglong start, unsigned signature)
{
    return -1;
}

static unsigned getcachesize(struct IO *cc)
{
    return 64 * 1024;
}

static const char *geterror(struct IO *cc)
{
    return "muh error";
}

static void *memalloc(struct IO *cc, size_t size)
{
    return malloc(size);
}

static void *memrealloc(struct IO *cc, void *mem, size_t newsize)
{
    return realloc(mem, newsize);
}

static void memfree(struct IO *cc, void *mem)
{
    free(mem);
}

static int progress(struct IO *cc, ulonglong cur, ulonglong max)
{
    return 1;
}

static longlong getfilesize(struct IO *cc) {
    off_t pos = ftello(cc->file);
    longlong ret = 0;
    fseeko(cc->file, 0, SEEK_END);
    ret = ftello(cc->file);
    fseeko(cc->file, pos, SEEK_SET);
}

void set_callbacks(IO *input)
{
    input->input.read = (int (*)(InputStream *,ulonglong,void *,int))ioread;
    input->input.scan = (longlong (*)(InputStream *,ulonglong,unsigned int))scan;
    input->input.getcachesize = (unsigned (*)(InputStream *cc))getcachesize;
    input->input.geterror = (const char *(*)(InputStream *))geterror;
    input->input.memalloc = (void *(*)(InputStream *,size_t))memalloc;
    input->input.memrealloc = (void *(*)(InputStream *,void *,size_t))memrealloc;
    input->input.memfree = (void (*)(InputStream *,void *))memfree;
    input->input.progress = (int (*)(InputStream *,ulonglong,ulonglong))progress;
    input->input.getfilesize = (longlong (*)(InputStream *))getfilesize;
}

IO* open_io(char *file)
{
    FILE *ifile = fopen(file, "rb");
    IO *ret;
    if (!ifile) { return NULL; }
    ret = calloc(1, sizeof(IO));
    if (!ret) { fclose(ifile); return NULL; }
    ret->file = ifile;
    ret->pos = 0;
    return ret;
}

int free_io(IO *io)
{
    fclose(io->file);
    free(io);
}

unsigned int get_width(TrackInfo *info)
{
    return info->AV.Video.PixelWidth;
}

unsigned int get_height(TrackInfo *info)
{
    return info->AV.Video.PixelHeight;
}
