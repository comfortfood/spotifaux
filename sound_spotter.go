package main

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_FFT_LENGTH = 4096

type soundSpotter struct {
	sampleRate      int
	WindowLength    int
	channels        int
	dbShingles      [][]float64
	inShingles      [][]float64
	dbPowers        []float64
	dbPowersCurrent []float64
	inPowers        []float64

	maxF int // Maximum number of source frames to extract (initial size of database)

	loFeature int
	hiFeature int
	dbBuf     []float64 // SoundSpotter pointer to PD internal buf
	bufLen    int64

	outputBuffer   []float64 // Matchers buffer
	hammingWinHalf []float64

	shingleSize int

	dbSize  int
	winner  int      // winning frame/shingle in seriesOfVectors match
	matcher *matcher // shingle matching algorithm

	pwr_abs_thresh float64 // don't match below this threshold

	wc   int
	cqtN int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func newSoundSpotter(sampleRate, WindowLength, channels int, dbBuf []float64, numFrames int64, cqtN int) *soundSpotter {

	s := &soundSpotter{
		sampleRate:     sampleRate,
		WindowLength:   WindowLength,
		channels:       channels,
		wc:             WindowLength * channels,
		loFeature:      3,
		hiFeature:      20,
		shingleSize:    4,
		winner:         -1,
		pwr_abs_thresh: 0.000001,
		dbSize:         0,
		cqtN:           cqtN,
	}

	s.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.makeHammingWin2()
	s.inShingles = make([][]float64, s.shingleSize)
	for i := 0; i < s.shingleSize; i++ {
		s.inShingles[i] = make([]float64, s.cqtN)
	}
	s.inPowers = make([]float64, s.shingleSize+1)

	s.outputBuffer = make([]float64, WindowLength*s.shingleSize*channels) // fix size at constructor ?

	s.dbShingles = make([][]float64, s.maxF)
	for i := 0; i < s.maxF; i++ {
		s.dbShingles[i] = make([]float64, s.cqtN)
	}
	s.dbPowers = make([]float64, s.maxF)
	s.dbPowersCurrent = make([]float64, s.maxF)
	s.matcher = &matcher{
		maxShingleSize: s.shingleSize,
		maxDBSize:      s.maxF,
		frameHashTable: make([]int, s.maxF),
	}
	s.matcher.resize(s.matcher.maxShingleSize, s.matcher.maxDBSize)

	s.channels = channels
	s.dbBuf = dbBuf
	s.bufLen = numFrames * int64(channels)
	if numFrames > int64(s.maxF)*int64(s.wc) {
		s.bufLen = int64(s.maxF) * int64(s.wc)
	}
	s.matcher.clearFrameQueue()

	return s
}

func (s *soundSpotter) getLengthSourceShingles() int {
	return int(math.Ceil(float64(s.bufLen) / (float64(s.wc))))
}

// This half hamming window is used for cross fading output buffers
func (s *soundSpotter) makeHammingWin2() {
	s.hammingWinHalf = make([]float64, s.WindowLength)
	for k := 0; k < s.WindowLength; k++ {
		s.hammingWinHalf[k] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(k)/float64(s.WindowLength*2-1))
	}
}

// Perform matching on shingle boundary
func (s *soundSpotter) match() {
	// zero output buffer in case we don't match anything
	s.zeroBuf(s.outputBuffer)
	// calculate powers for detecting silence and balancing output with input
	seriesMean(s.inPowers, s.shingleSize, s.shingleSize)
	if s.inPowers[0] > s.pwr_abs_thresh {
		// matched filter matching to get winning database shingle
		s.winner = s.matcher.match(s)
		if s.winner > -1 {
			// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
			// sqrt(env2) has already been calculated, only take sqrt(env1) here
			envFollow := 0.5
			alpha := envFollow*math.Sqrt(s.inPowers[0]/s.dbPowers[s.winner]) + (1 - envFollow)
			if s.winner > -1 {
				// Copy winning samples to output buffer, these could be further processed
				// MULTI-CHANNEL OUTPUT
				for p := 0; p < s.shingleSize*s.wc; p++ {
					output := alpha * s.dbBuf[s.winner*s.wc+p]
					if output > 1 {
						output = 1
					} else if output < -1 {
						output = -1
					}
					s.outputBuffer[p] = output
				}
			}
		}
	}
}

func (s *soundSpotter) syncOnShingleStart() {

	// update the power threshold data
	s.matcher.clearFrameQueue()

	// update the database norms for new parameters
	s.matcher.updateDatabaseNorms(s)
}

func (s *soundSpotter) zeroBuf(buf []float64) {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0.0
	}
}
