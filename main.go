package main

import (
    "fmt"
    "log"

    "github.com/dwbuiten/go-ffv1/matroska"
)

func main() {
    // this is a crappy demuxer
	mat, err := matroska.NewDemuxer("input.mkv")
    if err != nil {
        log.Fatalln(err)
    }
    defer mat.Close()

    extradata, err := mat.ReadCodecPrivate(0)
    if err != nil {
        log.Fatalln(err)
    }

    packet, track, err := mat.ReadPacket()
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Printf("extradata = %d packet = %d track = %d\n", len(extradata), len(packet), track)
}
