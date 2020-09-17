package main

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_MAX_SHINGLE_SZ = 32
const SS_FFT_LENGTH = 4096

type soundSpotter struct {
	sampleRate      int
	WindowLength    int
	channels        int
	dbShingles      ss_sample
	inShingle       *seriesOfVectors
	dbPowers        ss_sample
	dbPowersCurrent ss_sample
	inPowers        ss_sample

	maxF int // Maximum number of source frames to extract (initial size of database)

	loFeature       int
	hiFeature       int
	lastLoFeature   int       // persist feature parameters for database norming
	lastHiFeature   int       // persist feature parameters for database norming
	normsNeedUpdate bool      // flag to indicate that database norms are dirty
	dbBuf           ss_sample // SoundSpotter pointer to PD internal buf
	bufLen          int64

	outputBuffer   ss_sample // Matchers buffer
	lastAlpha      float64   // envelope follow of previous output
	hammingWinHalf ss_sample

	shingleSize     int
	lastShingleSize int

	dbSize     int
	lastWinner int      // previous winning frame
	winner     int      // winning frame/shingle in seriesOfVectors match
	matcher    *matcher // shingle matching algorithm

	pwr_abs_thresh float64 // don't match below this threshold

	LoK     int // Database start point marker
	HiK     int // Database end point marker
	lastLoK int
	lastHiK int
	wc      int
	cqtN    int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func newSoundSpotter(sampleRate, WindowLength, channels int, dbBuf ss_sample, numFrames int64, cqtN int) *soundSpotter {

	s := &soundSpotter{
		sampleRate:      sampleRate,
		WindowLength:    WindowLength,
		channels:        channels,
		wc:              WindowLength * channels,
		lastLoFeature:   -1,
		lastHiFeature:   -1,
		loFeature:       3,
		hiFeature:       20,
		lastShingleSize: -1,
		shingleSize:     4,
		lastWinner:      -1,
		winner:          -1,
		pwr_abs_thresh:  0.000001,
		lastLoK:         -1,
		lastHiK:         -1,
		dbSize:          0,
		lastAlpha:       0.0, // crossfade coefficient
		LoK:             0,
		HiK:             0,
		normsNeedUpdate: true,
		cqtN:            cqtN,
	}

	s.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.makeHammingWin2()
	s.inShingle = NewSeriesOfVectors(idxT(s.cqtN), idxT(SS_MAX_SHINGLE_SZ))
	s.inPowers = make(ss_sample, SS_MAX_SHINGLE_SZ)

	s.outputBuffer = make(ss_sample, WindowLength*SS_MAX_SHINGLE_SZ*channels) // fix size at constructor ?

	s.dbShingles = make(ss_sample, s.cqtN*s.maxF)
	s.dbPowers = make(ss_sample, s.maxF)
	s.dbPowersCurrent = make(ss_sample, s.maxF)
	s.matcher = &matcher{
		maxShingleSize: SS_MAX_SHINGLE_SZ,
		maxDBSize:      s.maxF,
		frameHashTable: make([]int, s.maxF),
	}
	s.matcher.resize(s.matcher.maxShingleSize, s.matcher.maxDBSize)

	if s.inShingle != nil && s.dbShingles != nil {
		s.zeroBuf(s.dbShingles)
		s.zeroBuf(s.inShingle.series)
		s.zeroBuf(s.inPowers)
		s.zeroBuf(s.dbPowers)
	}
	s.zeroBuf(s.outputBuffer)

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
	s.hammingWinHalf = make(ss_sample, s.WindowLength)
	for k := 0; k < s.WindowLength; k++ {
		s.hammingWinHalf[k] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(k)/float64(s.WindowLength*2-1))
	}
}

// Perform matching on shingle boundary
func (s *soundSpotter) match() {
	// zero output buffer in case we don't match anything
	s.zeroBuf(s.outputBuffer)
	// calculate powers for detecting silence and balancing output with input
	seriesMean(s.inPowers, idxT(s.shingleSize), idxT(s.shingleSize))
	if s.inPowers[0] > s.pwr_abs_thresh {
		s.lastWinner = s.winner // preserve previous state for cross-fading audio output
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
				// Cross-fade between current output shingle and one frame past end
				// of last winning shingle added to beginning of current
				if s.lastWinner > -1 && s.lastWinner < s.dbSize-s.shingleSize-1 {
					p := 0
					for w1 := 0; w1 < s.WindowLength; w1++ {
						w2 := s.WindowLength - 1 - w1
						for ; p < s.channels; p++ {
							output := alpha*s.dbBuf[s.winner*s.wc+p]*s.hammingWinHalf[w1] +
								s.lastAlpha*s.dbBuf[(s.lastWinner+s.shingleSize)*s.wc+p]*s.hammingWinHalf[w2]
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
			s.lastAlpha = alpha
		}
	}
}

func (s *soundSpotter) syncOnShingleStart() {

	s.normsNeedUpdate = s.normsNeedUpdate || !(s.lastLoFeature == s.loFeature &&
		s.lastHiFeature == s.hiFeature &&
		s.lastShingleSize == s.shingleSize &&
		s.lastLoK == s.LoK &&
		s.lastHiK == s.HiK)

	// update database statistics based on current search parameters
	if s.normsNeedUpdate {

		// update the power threshold data
		if s.shingleSize != s.lastShingleSize {
			copy(s.dbPowersCurrent, s.dbPowers)
			seriesMean(s.dbPowersCurrent, idxT(s.shingleSize), idxT(s.getLengthSourceShingles()))
			s.lastShingleSize = s.shingleSize
			s.matcher.clearFrameQueue()
		}

		// update the database norms for new parameters
		s.matcher.updateDatabaseNorms(s)

		// set the change indicators
		s.normsNeedUpdate = false
		s.lastLoFeature = s.loFeature
		s.lastHiFeature = s.hiFeature
		s.lastLoK = s.LoK
		s.lastHiK = s.HiK
	}
}

func (s *soundSpotter) zeroBuf(buf ss_sample) {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0.0
	}
}
