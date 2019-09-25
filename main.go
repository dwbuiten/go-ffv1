package main

import (
	"fmt"
	"log"

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
}
