package main

import (
	"fmt"
	"github.com/runningwild/go-fftw/fftw"
	"os"
	"spotifaux"
)

func main() {
	var fileName string
	if len(os.Args) > 1 {
		fileName = os.Args[1]
	} else {
		fileName = "/Users/wyatttall/git/spotifaux/db/slidingdown-16kHz.wav"
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

	rawInputSamps := make([]float64, spotifaux.Hop*(s.ShingleSize-1)+spotifaux.WindowLength)
	inputSamps := make([]float64, spotifaux.WindowLength)

	wav := spotifaux.NewWavWriter("out.wav")

	breakNext := false
	iter := 0
	for {
		if breakNext {
			break
		}

		nn := 0
		for ; iter > 0 && nn < spotifaux.WindowLength-spotifaux.Hop; nn++ {
			rawInputSamps[nn] = rawInputSamps[spotifaux.Hop*s.ShingleSize+nn]
		}

		for ; nn < (spotifaux.Hop*(s.ShingleSize-1) + spotifaux.WindowLength); nn++ {
			rawInputSamps[nn], err = inputSrc.Float64()
			if err != nil {
				breakNext = true
			}
		}

		s.SyncOnShingleStart() // update parameters at shingleStart

		for muxi := 0; muxi < s.ShingleSize; muxi++ {
			for nn := 0; nn < spotifaux.WindowLength; nn++ {
				inputSamps[nn] = rawInputSamps[muxi*spotifaux.Hop+nn]
			}
			// inputSamps holds the audio samples, convert inputSamps to outputFeatures (FFT buffer)
			e.ExtractVector(inputSamps, s.InShingles[muxi], &s.InPowers[muxi], fftIn, fftN, fftwPlan, fftOutN, fftComplex)
			// insert MFCC into SeriesOfVectors
			s.Matcher.Insert(s, muxi)
		}

		outputBuffer := s.Match()
		fmt.Printf("%d ", s.Winner)
		wav.WriteItems(outputBuffer)
		iter++
	}

	fmt.Printf("\n%d", iter)

	err = inputSrc.Close()
	if err != nil {
		panic(err)
	}

	err = wav.Close()
	if err != nil {
		panic(err)
	}
}
