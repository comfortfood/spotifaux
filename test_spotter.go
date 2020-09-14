package main

import (
	"errors"
	"fmt"
	"os"
)

const ITER_MAX = 1000
const N = 2048

func main() {
	var strbuf string
	if len(os.Args) > 1 {
		strbuf = os.Args[1]
	} else {
		strbuf = "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav"
	}
	src := newFixedSource()

	SS := newSoundSpotter(44100, N, 2)
	SF := &soundFile{}

	ret := SF.sfOpen(strbuf)
	if ret < 0 {
		panic(errors.New(fmt.Sprintf("Could not open %s", strbuf)))
	}
	SS.setAudioDatabaseBuf(SF.soundBuf, SF.numFrames, int(SF.info.Channels))

	// feature extraction
	SS.setStatus(EXTRACT)

	SS.run(N, nil, nil, nil)

	// SPOT mode test
	SS.setStatus(SPOT)

	inputSamps := make([]float64, N)
	outputFeatures := make([]float64, N)
	outputSamps := make([]float64, N)
	iter := 0
	nn := 0
	iterMax := ITER_MAX
	//iterMax = 76 // wyatt - bell source

	wav := NewWavWriter("out.wav")

	iter = 0
	for ; iter < iterMax; iter++ {
		nn = 0
		for nn < N {
			//TODO: wyatt says fixup with real random
			inputSamps[nn] = src.Float64() //(nn%512)/512.0f;
			outputFeatures[nn] = 0.0
			outputSamps[nn] = 0.0
			nn++
		}
		SS.run(N, inputSamps, outputFeatures, outputSamps)
		fmt.Printf("%d ", SS.reportResult())
		wav.WriteItems(outputSamps)
	}

	_ = src.Close()
	_ = wav.Close()
}
