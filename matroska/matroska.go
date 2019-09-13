package matroska

import (
	"fmt"
	"unsafe"
)

// #include "MatroskaParser.h"
// #include "io.h"
// InputStream *convert(IO* io) { return (InputStream *)io; }
import "C"

type Demuxer struct {
	m      *C.MatroskaFile
	errbuf *C.char
	io     *C.IO
}

func NewDemuxer(file string) (*Demuxer, error) {
	ret := new(Demuxer)

	ret.errbuf = (*C.char)(C.calloc(1, 1024))
	if ret.errbuf == nil {
		return nil, fmt.Errorf("could not allocate errbuf")
	}

	cfile := C.CString(file)
	defer C.free(unsafe.Pointer(cfile))
	ret.io = C.open_io(cfile)
	if ret.io == nil {
		C.free(unsafe.Pointer(ret.errbuf))
		return nil, fmt.Errorf("could not allocate io struct")
	}

	C.set_callbacks(ret.io)

	ret.m = C.mkv_OpenEx(C.convert(ret.io), 0, C.MKVF_AVOID_SEEKS, ret.errbuf, 1024)
	if ret.m == nil {
		reason := C.GoString(ret.errbuf)
		C.free(unsafe.Pointer(ret.errbuf))
		C.free_io(ret.io)
		return nil, fmt.Errorf("couldn't open matroska file: %s", reason)
	}

	return ret, nil
}

func (d *Demuxer) Close() {
	C.mkv_Close(d.m)
	C.free_io(d.io)
	C.free(unsafe.Pointer(d.errbuf))
}

func (d *Demuxer) ReadCodecPrivate(track uint) ([]byte, error) {
	info := C.mkv_GetTrackInfo(d.m, C.unsigned(track))

	if info == nil {
		reason := C.GoString(d.errbuf)
		return nil, fmt.Errorf("could not get codec private: %s", reason)
	}

	return C.GoBytes(unsafe.Pointer(uintptr(unsafe.Pointer(info.CodecPrivate))+40), C.int(info.CodecPrivateSize-40)), nil
}

func (d *Demuxer) ReadPacket() ([]byte, uint, error) {
	var track, size, flags C.unsigned
	var start, end, pos C.ulonglong
	var discard C.longlong
	var data *C.char

	mret := C.mkv_ReadFrame(d.m, 0, &track, &start, &end, &pos, &size, &data, &flags, &discard)
	if mret != 0 {
		reason := C.GoString(d.errbuf)
		return nil, 0, fmt.Errorf("could not get packet: %s", reason)
	}

	gdata := C.GoBytes(unsafe.Pointer(data), C.int(size))

	return gdata, uint(track), nil
}
