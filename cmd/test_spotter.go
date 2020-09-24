package main

import (
	"fmt"
	"github.com/runningwild/go-fftw/fftw"
	"os"
	"spotifaux"
)

const ITER_MAX = 1000

func main() {
	var fileName string
	if len(os.Args) > 1 {
		fileName = os.Args[1]
	} else {
		fileName = "/Users/wyatttall/git/spotifaux/recreate/bell-16kHz.wav"
	}
	var inputSrc spotifaux.Source
	//inputSrc = newFixedSource("/Users/wyatttall/git/BLAST/soundspotter/out")
	inputSrc = spotifaux.NewWavSource()
	sf, err := spotifaux.NewSoundFile(fileName)
	if err != nil {
		panic(err)
	}

	dbBuf := make([]float64, sf.Frames)
	_, err = sf.ReadFrames(dbBuf)
	if err != nil {
		panic(err)
	}

	fftN := spotifaux.SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1           // linear frequency power spectrum values (automatic)

	// FFTW memory allocation
	fftIn := fftw.NewArray(fftN)         // storage for FFT input
	fftComplex := fftw.NewArray(fftOutN) // storage for FFT output

	// FFTW plan caching
	fftwPlan := fftw.NewPlan(fftIn, fftComplex, fftw.Forward, fftw.Estimate)

	e := spotifaux.NewFeatureExtractor(spotifaux.SAMPLE_RATE, fftN, fftOutN)

	s := spotifaux.NewSoundSpotter(spotifaux.SAMPLE_RATE, dbBuf, sf.Frames, e.CqtN)

	e.ExtractSeriesOfVectors(s, fftIn, fftN, fftwPlan, fftOutN, fftComplex)

	inputSamps := make([]float64, spotifaux.WindowLength)
	iter := 0
	nn := 0
	iterMax := ITER_MAX
	iterMax = 140

	wav := spotifaux.NewWavWriter("out.wav")

	foundWinner := false

	iter = 0
	muxi := 0
	for ; iter < iterMax; iter++ {
		for nn = 0; nn < spotifaux.WindowLength; nn++ {
			//TODO: wyatt says fixup with real random
			inputSamps[nn] = inputSrc.Float64() //(nn%512)/512.0f;
		}

		if muxi == 0 {
			s.SyncOnShingleStart() // update parameters at shingleStart
		}

		// inputSamps holds the audio samples, convert inputSamps to outputFeatures (FFT buffer)
		e.ExtractVector(inputSamps, s.InShingles[muxi], &s.InPowers[muxi], fftIn, fftN, fftwPlan, fftOutN, fftComplex)
		// insert MFCC into SeriesOfVectors
		s.Matcher.Insert(s, muxi)
		// Do the matching at shingle end
		if muxi == s.ShingleSize-1 {
			outputBuffer := s.Match()
			if s.Winner != -1 || foundWinner {
				foundWinner = true
				fmt.Printf("%d ", s.Winner)
				wav.WriteItems(outputBuffer)
			}
		}

		// post-insert buffer multiplex increment
		muxi = (muxi + 1) % s.ShingleSize
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
