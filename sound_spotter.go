package main

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_MAX_SHINGLE_SZ = 32
const SS_FFT_LENGTH = 4096

type soundSpotter struct {
	sampleRate       int
	WindowLength     int
	numChannels      int
	MAX_SHINGLE_SIZE int // longest sequence of vectors
	dbShingles       *seriesOfVectors
	inShingle        *seriesOfVectors
	dbPowers         ss_sample
	dbPowersCurrent  ss_sample
	inPowers         ss_sample

	maxF int // Maximum number of source frames to extract (initial size of database)

	loFeature        int
	hiFeature        int
	lastLoFeature    int       // persist feature parameters for database norming
	lastHiFeature    int       // persist feature parameters for database norming
	normsNeedUpdate  bool      // flag to indicate that database norms are dirty
	audioDatabaseBuf ss_sample // SoundSpotter pointer to PD internal buf
	bufLen           int64

	audioOutputBuffer ss_sample // Matchers buffer
	muxi              int       // SHINGLEing overlap-add buffer multiplexer index
	lastAlpha         float64   // envelope follow of previous output
	hammingWin2       ss_sample

	shingleSize     int
	lastShingleSize int

	shingleHop       int
	dbSize           int
	lastWinner       int      // previous winning frame
	winner           int      // winning frame/shingle in seriesOfVectors match
	matcher          *matcher // shingle matching algorithm
	featureExtractor *featureExtractor

	pwr_abs_thresh float64 // don't match below this threshold

	LoK     int // Database start point marker
	HiK     int // Database end point marker
	lastLoK int
	lastHiK int

	envFollow float64 // amount to follow the input audio's energy envelope
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func newSoundSpotter(sampleRate, WindowLength, channels int, audioDatabaseBuf ss_sample, numFrames int64) *soundSpotter {

	s := &soundSpotter{
		sampleRate:      sampleRate,
		WindowLength:    WindowLength,
		numChannels:     channels,
		lastLoFeature:   -1,
		lastHiFeature:   -1,
		loFeature:       3,
		hiFeature:       20,
		lastShingleSize: -1,
		shingleSize:     4,
		shingleHop:      1,
		lastWinner:      -1,
		winner:          -1,
		pwr_abs_thresh:  0.000001,
		lastLoK:         -1,
		lastHiK:         -1,
		envFollow:       0,
		dbSize:          0,
		muxi:            0,   // multiplexer index
		lastAlpha:       0.0, // crossfade coefficient
		LoK:             0,
		HiK:             0,
		normsNeedUpdate: true,
	}

	s.featureExtractor = &featureExtractor{sampleRate: sampleRate, WindowLength: WindowLength, fftN: SS_FFT_LENGTH}
	s.featureExtractor.initializeFeatureExtractor()
	s.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.makeHammingWin2()
	s.inShingle = NewSeriesOfVectors(idxT(s.featureExtractor.cqtN), idxT(SS_MAX_SHINGLE_SZ))
	s.inPowers = make(ss_sample, SS_MAX_SHINGLE_SZ)

	s.audioOutputBuffer = make(ss_sample, WindowLength*SS_MAX_SHINGLE_SZ*channels) // fix size at constructor ?

	s.dbShingles = NewSeriesOfVectors(idxT(s.featureExtractor.cqtN), idxT(s.maxF))
	s.dbPowers = make(ss_sample, s.maxF)
	s.dbPowersCurrent = make(ss_sample, s.maxF)
	s.matcher = &matcher{
		maxShingleSize: SS_MAX_SHINGLE_SZ,
		maxDBSize:      s.maxF,
		frameHashTable: make([]int, s.maxF),
	}
	s.matcher.resize(s.matcher.maxShingleSize, s.matcher.maxDBSize)

	if s.inShingle != nil && s.dbShingles != nil {
		s.zeroBuf(s.dbShingles.series)
		s.zeroBuf(s.inShingle.series)
		s.zeroBuf(s.inPowers)
		s.zeroBuf(s.dbPowers)
	}
	s.zeroBuf(s.audioOutputBuffer)

	s.numChannels = channels
	s.audioDatabaseBuf = audioDatabaseBuf
	s.bufLen = numFrames * int64(channels)
	if numFrames > int64(s.maxF)*int64(s.WindowLength)*int64(channels) {
		s.bufLen = int64(s.maxF) * int64(s.WindowLength) * int64(channels)
	}
	s.matcher.clearFrameQueue()

	return s
}

func (s *soundSpotter) getLengthSourceShingles() int {
	return int(math.Ceil(float64(s.bufLen) / (float64(s.numChannels) * float64(s.WindowLength))))
}

// This half hamming window is used for cross fading output buffers
func (s *soundSpotter) makeHammingWin2() {
	s.hammingWin2 = make(ss_sample, s.WindowLength)
	TWO_PI := 2 * 3.14159265358979
	oneOverWinLen2m1 := 1.0 / float64(s.WindowLength*2-1)
	for k := 0; k < s.WindowLength; k++ {
		s.hammingWin2[k] = 0.54 - 0.46*math.Cos(TWO_PI*float64(k)*oneOverWinLen2m1)
	}
}

func (s *soundSpotter) spot(n int, inputSamps, outputFeatures, outputSamps ss_sample) {
	if s.muxi == 0 {
		s.syncOnShingleStart() // update parameters at shingleStart
	}
	// inputSamps holds the audio samples, convert inputSamps to outputFeatures (FFT buffer)
	s.featureExtractor.extractVector(n, inputSamps, outputFeatures, &s.inPowers[s.muxi])
	// insert MFCC into SeriesOfVectors
	copy(s.inShingle.getCol(idxT(s.muxi)), outputFeatures[:s.inShingle.rows])
	// insert shingles into Matcher
	s.matcher.insert(s)
	// post-insert buffer multiplex increment
	s.muxi = (s.muxi + 1) % s.shingleSize
	// Do the matching at shingle end
	if s.muxi == 0 {
		s.match()
	}
	// generate current frame's output sample and update everything
	p2 := s.muxi * s.WindowLength * s.numChannels // so that this is synchronized on match boundaries
	p1 := 0
	nn := s.WindowLength * s.numChannels
	for ; nn > 0; nn-- {
		outputSamps[p1] = s.audioOutputBuffer[p2] // multi-channel output
		p1++
		p2++
	}
}

// Perform matching on shingle boundary
func (s *soundSpotter) match() {
	// zero output buffer in case we don't match anything
	s.zeroBuf(s.audioOutputBuffer)
	// calculate powers for detecting silence and balancing output with input
	seriesMean(s.inPowers, idxT(s.shingleSize), idxT(s.shingleSize))
	if s.inPowers[0] > s.pwr_abs_thresh {
		s.lastWinner = s.winner // preserve previous state for cross-fading audio output
		// matched filter matching to get winning database shingle
		s.winner = s.matcher.match(s)
		if s.winner > -1 {
			p := 0 // MULTI-CHANNEL OUTPUT
			q := s.winner * s.WindowLength * s.numChannels
			env1 := s.inPowers[0]
			env2 := s.dbPowers[s.winner]
			// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
			// sqrt(env2) has already been calculated, only take sqrt(env1) here
			alpha := s.envFollow*math.Sqrt(env1/env2) + (1 - s.envFollow)
			if s.winner > -1 {
				// Copy winning samples to output buffer, these could be further processed
				nn := s.WindowLength * s.shingleSize * s.numChannels
				for ; nn > 0; nn-- {
					s.audioOutputBuffer[p] = alpha * s.audioDatabaseBuf[q]
					p++
					q++
				}
				//// Cross-fade between current output shingle and one frame past end
				//// of last winning shingle added to beginning of current
				//if s.lastWinner > -1 && s.lastWinner < s.dbSize-s.shingleSize-1 {
				//	p = 0                                                               // first scanning pointer
				//	q = (s.lastWinner + s.shingleSize) * s.WindowLength * s.numChannels // one past end of last output buffer
				//	p1 := s.winner * s.WindowLength * s.numChannels                     // this winner (first frame)
				//	w1 := 0                                                             // forwards half-hamming pointer
				//	w2 := s.WindowLength - 1                                            // backwards half-hamming pointer
				//	nn = s.WindowLength                                                 // first audio output buffer only
				//	c := 0
				//	for ; nn > 0; nn-- {
				//		c = s.numChannels
				//		for ; c > 0; c-- {
				//			s.audioOutputBuffer[p] = alpha*s.audioDatabaseBuf[p1]*s.hammingWin2[w1] + s.lastAlpha*s.audioDatabaseBuf[q]*s.hammingWin2[w2]
				//			p++
				//			p1++
				//			q++
				//		}
				//		w1++
				//		w2--
				//	}
				//}
			}
			s.lastAlpha = alpha
		}
	}
}

func (s *soundSpotter) syncOnShingleStart() {
	if s.muxi == 0 { // parameters and statistics must only change on match boundaries
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
			s.matcher.updateDatabaseNorms(s.dbShingles, s.shingleSize, s.getLengthSourceShingles(),
				s.loFeature, s.hiFeature, s.LoK, s.HiK)
			// set the change indicators
			s.normsNeedUpdate = false
			s.lastLoFeature = s.loFeature
			s.lastHiFeature = s.hiFeature
			s.lastLoK = s.LoK
			s.lastHiK = s.HiK
		}
	}
}

func (s *soundSpotter) zeroBuf(buf ss_sample) {
	for i := 0; i < len(buf); i++ {
		buf[i] = 0.0
	}
}
