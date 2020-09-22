package main

import (
	"fmt"
	"github.com/runningwild/go-fftw/fftw"
	"os"
)

const ITER_MAX = 1000
const WindowLength = 2048

func main() {
	var fileName string
	if len(os.Args) > 1 {
		fileName = os.Args[1]
	} else {
		fileName = "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav"
		//fileName = "/Users/wyatttall/git/spotifaux/bell2.wav"
	}
	var inputSrc source
	inputSrc = newFixedSource("/Users/wyatttall/git/BLAST/soundspotter/out")
	//inputSrc = newWavSource()
	sf, err := newSoundFile(fileName)
	if err != nil {
		panic(err)
	}

	dbBuf := make([]float64, sf.frames*int64(sf.channels))
	_, err = sf.ReadFrames(dbBuf)
	if err != nil {
		panic(err)
	}

	fftN := SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1 // linear frequency power spectrum values (automatic)

	// FFTW memory allocation
	fftIn := fftw.NewArray(fftN)                 // storage for FFT input
	fftComplex := fftw.NewArray(fftOutN)         // storage for FFT output
	fftPowerSpectrum := make([]float64, fftOutN) // storage for FFT power spectrum

	// FFTW plan caching
	fftwPlan := fftw.NewPlan(fftIn, fftComplex, fftw.Forward, fftw.Estimate)

	e := newFeatureExtractor(44100, WindowLength, fftN, fftOutN)

	s := newSoundSpotter(44100, WindowLength, sf.channels, dbBuf, sf.frames, e.cqtN)

	e.extractSeriesOfVectors(s, fftIn, fftN, fftwPlan, fftOutN, fftComplex, fftPowerSpectrum)

	inputSamps := make([]float64, WindowLength*sf.channels)
	outputFeatures := make([]float64, WindowLength*sf.channels)
	iter := 0
	nn := 0
	iterMax := ITER_MAX
	//iterMax = 76

	wav := NewWavWriter("out.wav")

	foundWinner := false

	iter = 0
	muxi := 0
	for ; iter < iterMax; iter++ {
		for nn = 0; nn < WindowLength; nn++ {
			//TODO: wyatt says fixup with real random
			inputSamps[nn] = inputSrc.Float64() //(nn%512)/512.0f;
			outputFeatures[nn] = 0.0
		}
		if s.dbSize == 0 || s.bufLen == 0 {
			break
		}

		if muxi == 0 {
			s.syncOnShingleStart() // update parameters at shingleStart
		}

		// inputSamps holds the audio samples, convert inputSamps to outputFeatures (FFT buffer)
		e.extractVector(inputSamps, outputFeatures, &s.inPowers[muxi], fftIn, fftN, fftwPlan, fftOutN, fftComplex, fftPowerSpectrum)
		// insert MFCC into SeriesOfVectors
		copy(s.inShingles[muxi], outputFeatures[:s.cqtN])
		// insert shingles into Matcher
		s.matcher.insert(s, muxi)
		// Do the matching at shingle end
		if muxi == (s.shingleSize - 1) {
			s.match()
			if s.winner != -1 || foundWinner {
				foundWinner = true
				fmt.Printf("%d ", s.winner)
				wav.WriteItems(s.outputBuffer)
			}
		}

		// post-insert buffer multiplex increment
		muxi = (muxi + 1) % s.shingleSize
	}

	err = inputSrc.Close()
	if err != nil {
		panic(err)
	}

	err = wav.Close()
	if err != nil {
		panic(err)
	}
}
