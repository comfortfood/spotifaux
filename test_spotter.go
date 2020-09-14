package main

import (
	"fmt"
	"os"
)

const ITER_MAX = 1000
const N = 2048
const MAXRANDINT = 32767.0

// test0001: open sound file, set SoundSpotter buffer to SoundFile buffer
func test0001(SS *soundSpotter, SF *soundFile, strbuf string) int {
	retval := SF.sfOpen(strbuf)
	if retval < 0 {
		fmt.Printf("Could not open %s\n", strbuf)
		return 1
	}
	SS.setAudioDatabaseBuf(SF.getSoundBuf(), SF.getBufLen(), SF.getNumChannels())
	return 0
}

// test0002: perform feature extraction on a test sound
func test0002(SS *soundSpotter, strbuf string) int {
	SS.setStatus(EXTRACT)
	fmt.Printf("EXTRACTING %s...", strbuf)

	SS.run(N, nil, nil, nil, nil)
	SS.setStatus(SPOT)
	fmt.Printf("\nDONE.\n")
	return 0
}

func main() {
	var strbuf string
	if len(os.Args) > 1 {
		strbuf = os.Args[1]
	} else {
		strbuf = "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav"
	}
	r := newInputSource()

	SS := newSoundSpotter(44100, N, 2)
	SF := &soundFile{}

	ret := test0001(SS, SF, strbuf)

	// feature extraction
	ret += test0002(SS, strbuf)

	// SPOT mode test
	ret += test0003(SS, r)

	r.Close()
}

// runSpotter: run spotter
func runSpotter(SS *soundSpotter, r inputSource) int {
	inputFeatures := make([]float64, N)
	inputSamps := make([]float64, N)
	outputFeatures := make([]float64, N)
	outputSamps := make([]float64, N)
	iter := 0
	nn := 0
	iterMax := ITER_MAX

	iter = 0
	for ; iter < iterMax; iter++ {
		nn = 0
		for nn < N {
			inputFeatures[nn] = 0.0
			//TODO: wyatt says fixup with real random
			inputSamps[nn] = r.Float64() //(nn%512)/512.0f;
			outputFeatures[nn] = 0.0
			outputSamps[nn] = 0.0
			nn++
		}
		SS.run(N, inputFeatures, inputSamps, outputFeatures, outputSamps)
		fmt.Printf("%d ", SS.reportResult())
	}
	return 0
}

// test0003: spot mode test
func test0003(SS *soundSpotter, r inputSource) int {
	fmt.Printf("SPOT...")
	SS.setStatus(SPOT)
	return runSpotter(SS, r)
}
