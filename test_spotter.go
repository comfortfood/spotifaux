package main

import (
	"fmt"
	"os"
)

const ITER_MAX = 1000
const N = 2048

func main() {
	var fileName string
	if len(os.Args) > 1 {
		fileName = os.Args[1]
	} else {
		fileName = "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav"
		//fileName = "/Users/wyatttall/git/spotifaux/bell2.wav"
	}
	var src source
	src = newFixedSource()
	//src = newWavSource()
	sf, err := newSoundFile(fileName)
	if err != nil {
		panic(err)
	}

	audioDatabaseBuf := make([]float64, sf.frames*int64(sf.channels))
	_, err = sf.ReadFrames(audioDatabaseBuf)
	if err != nil {
		panic(err)
	}

	s := newSoundSpotter(44100, N, sf.channels, audioDatabaseBuf, sf.frames)
	s.featureExtractor.extractSeriesOfVectors(s)

	inputSamps := make([]float64, N*sf.channels)
	outputFeatures := make([]float64, N*sf.channels)
	outputSamps := make([]float64, N*sf.channels)
	iter := 0
	nn := 0
	iterMax := ITER_MAX
	//iterMax = 76

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

	err = src.Close()
	if err != nil {
		panic(err)
	}
	err = wav.Close()
	if err != nil {
		panic(err)
	}
}
