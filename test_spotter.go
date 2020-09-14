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

	sf := &soundFile{}
	ret := sf.sfOpen(strbuf)
	if ret < 0 {
		panic(errors.New(fmt.Sprintf("Could not open %s", strbuf)))
	}

	s := newSoundSpotter(44100, N, 2, sf.soundBuf, sf.numFrames, int(sf.info.Channels))
	s.featureExtractor.extractSeriesOfVectors(s)

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
		for nn = 0; nn < N; nn++ {
			//TODO: wyatt says fixup with real random
			inputSamps[nn] = src.Float64() //(nn%512)/512.0f;
			outputFeatures[nn] = 0.0
			outputSamps[nn] = 0.0
		}
		if s.dbSize == 0 || s.bufLen == 0 {
			break
		}
		s.spot(N, inputSamps, outputFeatures, outputSamps)
		fmt.Printf("%d ", s.winner)
		wav.WriteItems(outputSamps)
	}

	_ = src.Close()
	_ = wav.Close()
}
