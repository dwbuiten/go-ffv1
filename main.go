package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/dwbuiten/go-ffv1/ffv1"
	"github.com/dwbuiten/go-ffv1/matroska"
)

func main() {
	// this is a crappy demuxer
	mat, err := matroska.NewDemuxer("input.mkv")
	if err != nil {
		log.Fatalln(err)
	}
	defer mat.Close()

	width, height, err := mat.GetDimensions(0)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("Encode is %dx%d\n", width, height)

	extradata, err := mat.ReadCodecPrivate(0)
	if err != nil {
		log.Fatalln(err)
	}

	packet, track, err := mat.ReadPacket()
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("extradata = %d packet = %d track = %d\n", len(extradata), len(packet), track)

	d, err := ffv1.NewDecoder(extradata, width, height)
	if err != nil {
		log.Fatalln(err)
	}

	frame, err := d.DecodeFrame(packet)
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
