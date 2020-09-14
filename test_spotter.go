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

	SS.resetShingles(SS.getLengthSourceShingles())
	SS.resetMatchBuffer()
	SS.dbSize = SS.featureExtractor.extractSeriesOfVectors(SS)
	SS.normsNeedUpdate = true

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
		if SS.dbSize == 0 || SS.bufLen == 0 {
			break
		}
		SS.spot(N, inputSamps, outputFeatures, outputSamps)
		fmt.Printf("%d ", SS.winner)
		wav.WriteItems(outputSamps)
	}

	_ = src.Close()
	_ = wav.Close()
}
