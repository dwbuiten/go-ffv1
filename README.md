FFV1 Decoder in Go
---

This repo contains an FFV1 Version 3 decoder implemented from draft-ietf-cellar-ffv1.

The reason for this project was to test how good the specification was, and indeed, during
the development of this, several issues were unearthed. The secondary goal was to write a
readable, commented, good reference; obviousness and spec-similarity over speed, struct
members that match the spec, etc.

... Well, at least the former goal was met.

As such, there's a lot of non-idiomatic Go in here, and a lot of gross argument passing
that should be refactored out into the contexts, stuff that should be interfaces instead
of code generation etc... one day. Any day now. If we keep waiting, it may happen. Surely.

TODO
---

* 16-bit RGB (requires 17-bit scratch buffer)

Example of Decoding FFV1 in Matroska
---

```Go
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/dwbuiten/go-ffv1/ffv1"
	"github.com/dwbuiten/matroska"
)

func main() {
	f, err := os.Open("input.mkv")
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	mat, err := matroska.NewDemuxer(f)
	if err != nil {
		log.Fatalln(err)
	}
	defer mat.Close()

	// Assuming track 0 is video because lazy.
	ti, err := mat.GetTrackInfo(0)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Encode is %dx%d\n", ti.Video.PixelWidth, ti.Video.PixelHeight)

	extradata := ti.CodecPrivate
	if strings.Contains(ti.CodecID, "VFW") {
		extradata = extradata[40:] // As per Matroska spec for VFW CodecPrivate
	}

	d, err := ffv1.NewDecoder(extradata, ti.Video.PixelWidth, ti.Video.PixelHeight)
	if err != nil {
		log.Fatalln(err)
	}

	file, err := os.Create("test.raw")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	for {
		packet, err := mat.ReadPacket()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("extradata = %d packet = %d track = %d\n", len(extradata), len(packet.Data), packet.Track)
		if packet.Track != 0 {
			continue
		}

		frame, err := d.DecodeFrame(packet.Data)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Frame decoded at %dx%d\n", frame.Width, frame.Height)

		if frame.BitDepth == 8 {
			err = binary.Write(file, binary.LittleEndian, frame.Buf[0])
			if err != nil {
				log.Fatalln(err)
			}
			err = binary.Write(file, binary.LittleEndian, frame.Buf[1])
			if err != nil {
				log.Fatalln(err)
			}
			err = binary.Write(file, binary.LittleEndian, frame.Buf[2])
			if err != nil {
				log.Fatalln(err)
			}
		} else {
			err = binary.Write(file, binary.LittleEndian, frame.Buf16[0])
			if err != nil {
				log.Fatalln(err)
			}
			err = binary.Write(file, binary.LittleEndian, frame.Buf16[1])
			if err != nil {
				log.Fatalln(err)
			}
			err = binary.Write(file, binary.LittleEndian, frame.Buf16[2])
			if err != nil {
				log.Fatalln(err)
			}
		}
	}
	fmt.Println("Done.")
}
```
