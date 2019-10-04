package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

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

	ti, err := mat.GetTrackInfo(0)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Encode is %dx%d\n", ti.Video.PixelWidth, ti.Video.PixelHeight)

	extradata := ti.CodecPrivate[40:]

	packet, err := mat.ReadPacket()
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("extradata = %d packet = %d track = %d\n", len(extradata), len(packet.Data), packet.Track)

	d, err := ffv1.NewDecoder(extradata, ti.Video.PixelWidth, ti.Video.PixelHeight)
	if err != nil {
		log.Fatalln(err)
	}

	frame, err := d.DecodeFrame(packet.Data)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Frame decoded at %dx%d\n", frame.Width, frame.Height)

	file, err := os.Create("t.yuv")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	y := bytes.NewBuffer(frame.Buf[0])
	u := bytes.NewBuffer(frame.Buf[1])
	v := bytes.NewBuffer(frame.Buf[2])

	_, err = io.Copy(file, y)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = io.Copy(file, u)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = io.Copy(file, v)
	if err != nil {
		log.Fatalln(err)
	}
}
