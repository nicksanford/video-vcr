package main

import (
	"fmt"
	"os"

	vcr "github.com/nicksanford/video-vcr"
	"go.viam.com/rdk/logging"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("usage: %s <db file path>\n", os.Args[0])
		os.Exit(1)
	}
	r, err := vcr.NewRecorder(os.Args[1], logging.NewLogger("h265recorder"))
	if err != nil {
		panic(err.Error())
	}
	if err := r.Init(vcr.CodecH264, 2560, 1440); err != nil {
		panic(err.Error())
	}

	if err := r.Packet([]byte{11, 12, 13}, 66, 66, true); err != nil {
		panic(err.Error())
	}
	if err := r.Close(); err != nil {
		panic(err.Error())
	}
}
