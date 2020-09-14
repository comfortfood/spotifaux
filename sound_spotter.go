package main

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_MAX_SHINGLE_SZ = 32
const SS_FFT_LENGTH = 4096

type SoundSpotterStatus int

const (
	STOP SoundSpotterStatus = iota
	EXTRACT
	SPOT
)

type soundSpotter struct {
	sampleRate          int
	WindowLength        int
	numChannels         int
	MAX_SHINGLE_SIZE    int // longest sequence of vectors
	sourceShingles      *seriesOfVectors
	inShingle           *seriesOfVectors
	sourcePowers        *seriesOfVectors
	sourcePowersCurrent *seriesOfVectors
	inPowers            *seriesOfVectors

	maxF int // Maximum number of source frames to extract (initial size of database)

	minASB           int
	maxASB           int
	lastLoFeature    int // persist feature parameters for database norming
	lastHiFeature    int // persist feature parameters for database norming
	ifaceLoFeature   int // store interface values to change on shingle boundary
	ifaceHiFeature   int
	normsNeedUpdate  bool // flag to indicate that database norms are dirty
	audioDisabled    int  // whether to process audio
	basisWidth       int
	audioDatabaseBuf ss_sample // SoundSpotter pointer to PD internal buf
	bufLen           int64

	audioOutputBuffer ss_sample // Matchers buffer
	muxi              int       // SHINGLEing overlap-add buffer multiplexer index
	lastAlpha         float64   // envelope follow of previous output
	hammingWin2       ss_sample

	shingleSize      int
	lastShingleSize  int
	ifaceShingleSize int

	shingleHop       int
	XPtr             int
	lastWinner       int      // previous winning frame
	winner           int      // winning frame/shingle in seriesOfVectors match
	matcher          *matcher // shingle matching algorithm
	featureExtractor *featureExtractor

	pwr_abs_thresh float64 // don't match below this threshold

	LoK      int // Database start point marker
	HiK      int // Database end point marker
	lastLoK  int
	lastHiK  int
	ifaceLoK int
	ifaceHiK int

	envFollow float64 // amount to follow the input audio's energy envelope

	soundSpotterStatus SoundSpotterStatus
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func newSoundSpotter(sampleRate int, WindowLength int, numChannels int) *soundSpotter {

	ss := &soundSpotter{
		sampleRate:         sampleRate,
		WindowLength:       WindowLength,
		numChannels:        numChannels,
		lastLoFeature:      -1,
		lastHiFeature:      -1,
		ifaceLoFeature:     3,
		ifaceHiFeature:     20,
		lastShingleSize:    -1,
		ifaceShingleSize:   4,
		shingleHop:         1,
		lastWinner:         -1,
		winner:             -1,
		pwr_abs_thresh:     0.000001,
		LoK:                -1,
		HiK:                -1,
		lastLoK:            -1,
		lastHiK:            -1,
		soundSpotterStatus: STOP,
		//minASB:             5,
		//queueSize:		  0,
		//envFollow:          0,
	}

	ss.featureExtractor = &featureExtractor{sampleRate: sampleRate, WindowLength: WindowLength, fftN: SS_FFT_LENGTH}
	ss.featureExtractor.initializeFeatureExtractor()
	ss.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	ss.makeHammingWin2()
	ss.MAX_SHINGLE_SIZE = SS_MAX_SHINGLE_SZ
	ss.inShingle = NewSeriesOfVectors(idxT(ss.featureExtractor.cqtN), idxT(ss.MAX_SHINGLE_SIZE))
	ss.inPowers = NewSeriesOfVectors(idxT(ss.MAX_SHINGLE_SIZE), idxT(1))

	ss.audioOutputBuffer = make(ss_sample, WindowLength*ss.MAX_SHINGLE_SIZE*numChannels) // fix size at constructor ?
	ss.resetShingles(ss.maxF)                                                            // Some default number of shingles
	return ss
}

func (s *soundSpotter) getLengthSourceShingles() int {
	return int(math.Ceil(float64(s.bufLen) / (float64(s.numChannels) * float64(s.WindowLength))))
}

// Implement triggers based on state transitions
// Status changes to EXTRACT
// require memory checking and possible allocation
//
// FIXME: Memory allocation discipline is suspicious
// Sometimes the audio buffer is user allocated, other times it is auto allocated
// EXTRACT: requires a source buffer (audioDatabaseBuf) to be provided, allocates database
func (s *soundSpotter) setStatus(stat SoundSpotterStatus) int {

	if stat == s.soundSpotterStatus {
		return 0 // fail if no new state
	}

	switch stat {

	case SPOT:
		break

	case EXTRACT:
		if stat == EXTRACT && s.audioDatabaseBuf == nil {
			return 0 // fail if no audio to extract (user allocated audio buffer)
		}
		break

	case STOP:

	default:
		return 0
	}

	s.soundSpotterStatus = stat

	return 1
}

func (s *soundSpotter) setLoDataLoc(l float32) {
	l *= float32(s.sampleRate) / float32(s.WindowLength)
	if l < 0 {
		l = 0
	} else if l > float32(s.getLengthSourceShingles()) {
		l = float32(s.getLengthSourceShingles())
	}
	s.ifaceLoK = int(l)
}

func (s *soundSpotter) setHiDataLoc(h float32) {
	h *= float32(s.sampleRate) / float32(s.WindowLength)
	if h < 0 {
		h = 0
	} else if h > float32(s.getLengthSourceShingles()) {
		h = float32(s.getLengthSourceShingles())
	}
	s.ifaceHiK = int(h)
}

func (s *soundSpotter) setAudioDatabaseBuf(b ss_sample, l int64, channels int) {
	s.numChannels = channels
	s.audioDatabaseBuf = b
	l = l * int64(channels)
	if l > int64(s.maxF)*int64(s.WindowLength)*int64(channels) {
		l = int64(s.maxF) * int64(s.WindowLength) * int64(channels)
	}
	s.bufLen = l
}

// Buffer reset for EXTRACT
func (s *soundSpotter) resetBufPtrs() {
	s.XPtr = 0
	//if s.inShingle != nil && s.sourceShingles != nil {
	//	// zero-out source shingles
	//	v := s.sourceShingles
	//	s.zeroBuf(v.series, v.rows*v.columns)
	//	// zero-out inShingle
	//	v = s.inShingle
	//	s.zeroBuf(v.series, v.rows*v.columns)
	//
	//	// zero-out power sequences
	//	s.zeroBuf(s.inPowers.getCol(0), s.inPowers.rows)
	//	s.zeroBuf(s.sourcePowers.getCol(0), s.sourcePowers.rows)
	//}
	//
	//// audio output buffer
	//s.zeroBuf(s.audioOutputBuffer, uint64(SS_MAX_SHINGLE_SZ*s.WindowLength*s.numChannels))
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

func (s *soundSpotter) run(n int, inputSamps, outputFeatures, outputSamps ss_sample) {
	switch s.soundSpotterStatus {
	case STOP:
		i := 0
		for ; n > 0; n-- {
			outputSamps[i] = 0
			i++
		}
		return

	case EXTRACT:
		s.resetShingles(s.getLengthSourceShingles())
		s.resetMatchBuffer()
		s.XPtr = s.featureExtractor.extractSeriesOfVectors(s)
		s.normsNeedUpdate = true
		s.setStatus(SPOT)
		return

	case SPOT:
		s.spot(n, inputSamps, outputFeatures, outputSamps)
		return
	}
}

func (s *soundSpotter) spot(n int, inputSamps, outputFeatures, outputSamps ss_sample) {
	if s.checkExtracted() == 0 {
		return // features extracted?
	}
	if s.muxi == 0 {
		s.syncOnShingleStart() // update parameters at shingleStart
	}
	// inputSamps holds the audio samples, convert inputSamps to outputFeatures (FFT buffer)
	s.featureExtractor.extractVectorFromMono(n, inputSamps, outputFeatures, &s.inPowers.series[s.muxi])
	// insert MFCC into SeriesOfVectors
	s.inShingle.insert(outputFeatures, idxT(s.muxi))
	// insert shingles into Matcher
	s.matcher.insert(s.inShingle, s.shingleSize, s.sourceShingles, s.XPtr, s.muxi, s.minASB, s.maxASB, s.LoK, s.HiK)
	// post-insert buffer multiplex increment
	s.muxi = s.incrementMultiplexer(s.muxi, s.shingleSize)
	// Do the matching at shingle end
	if s.muxi == 0 {
		s.match()
	}
	// generate current frame's output sample and update everything
	s.updateAudioOutputBuffers(outputSamps)
}

func (s *soundSpotter) incrementMultiplexer(multiplex, sz int) int {
	return (multiplex + 1) % sz
}

func (s *soundSpotter) checkExtracted() int {
	if s.XPtr == 0 || s.bufLen == 0 {
		s.setStatus(STOP)
		return 0
	} else {
		return 1
	}
}

// Perform matching on shingle boundary
func (s *soundSpotter) match() {
	// zero output buffer in case we don't match anything
	s.zeroBuf(s.audioOutputBuffer, uint64(s.WindowLength*s.shingleSize*s.numChannels))
	// calculate powers for detecting silence and balancing output with input
	seriesMean(s.inPowers.series, idxT(s.shingleSize), idxT(s.shingleSize))
	inPwMn := s.inPowers.series[0]
	if inPwMn > s.pwr_abs_thresh {
		s.lastWinner = s.winner // preserve previous state for cross-fading audio output
		// matched filter matching to get winning database shingle
		s.winner = s.matcher.match(s.shingleSize, s.XPtr, s.LoK, s.HiK,
			inPwMn, s.sourcePowersCurrent.series, s.pwr_abs_thresh)
		if s.winner > -1 {
			s.sampleBuf() // fill output audioOutputBuffer with matched source frames
		}
	}
}

// update() take care of the current DSP frame output buffer
//
// pre-conditions
//      multiplexer index (muxi) must be an integer (integral type)
// post-conditions
//      n samples ready in outs2 buffer
func (s *soundSpotter) updateAudioOutputBuffers(outputSamps ss_sample) {
	p2 := s.muxi * s.WindowLength * s.numChannels // so that this is synchronized on match boundaries
	p1 := 0
	nn := s.WindowLength * s.numChannels
	for ; nn > 0; nn-- {
		outputSamps[p1] = s.audioOutputBuffer[p2] // multi-channel output
		p1++
		p2++
	}
}

func (s *soundSpotter) reportResult() int {
	return s.winner
}

// sampleBuf() copy n*shingleHop to audioOutputBuffer from best matching segment in source buffer audioDatabaseBuf[]
func (s *soundSpotter) sampleBuf() {
	p := 0 // MULTI-CHANNEL OUTPUT
	q := s.winner * s.WindowLength * s.numChannels
	env1 := s.inPowers.series[0]
	env2 := s.sourcePowers.series[s.winner]
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
		// Cross-fade between current output shingle and one frame past end
		// of last winning shingle added to beginning of current
		if s.lastWinner > -1 && s.lastWinner < s.XPtr-s.shingleSize-1 {
			p = 0                                                               // first scanning pointer
			q = (s.lastWinner + s.shingleSize) * s.WindowLength * s.numChannels // one past end of last output buffer
			p1 := s.winner * s.WindowLength * s.numChannels                     // this winner (first frame)
			w1 := 0                                                             // forwards half-hamming pointer
			w2 := s.WindowLength - 1                                            // backwards half-hamming pointer
			nn = s.WindowLength                                                 // first audio output buffer only
			c := 0
			for ; nn > 0; nn-- {
				c = s.numChannels
				for ; c > 0; c-- {
					s.audioOutputBuffer[p] = alpha*s.audioDatabaseBuf[p1]*s.hammingWin2[w1] + s.lastAlpha*s.audioDatabaseBuf[q]*s.hammingWin2[w2]
					p++
					p1++
					q++
				}
				w1++
				w2--
			}
		}
	}
	s.lastAlpha = alpha
}

func (s *soundSpotter) resetShingles(newSize int) int {
	if newSize > s.maxF {
		newSize = s.maxF
	}

	if s.sourceShingles == nil {
		s.sourceShingles = NewSeriesOfVectors(idxT(s.featureExtractor.cqtN), idxT(newSize))
		s.sourcePowers = NewSeriesOfVectors(idxT(newSize), idxT(1))
		s.sourcePowersCurrent = NewSeriesOfVectors(idxT(newSize), idxT(1))
		s.MAX_SHINGLE_SIZE = SS_MAX_SHINGLE_SZ
		s.matcher = &matcher{
			maxShingleSize: s.MAX_SHINGLE_SIZE,
			maxDBSize:      newSize,
			frameHashTable: make([]int, newSize),
		}
		s.matcher.resize(s.matcher.maxShingleSize, s.matcher.maxDBSize)
	}

	s.resetBufPtrs() // fill shingles with zeros and set buffer pointers to zero
	return newSize
}

// perform reset on new sound file load or extract
func (s *soundSpotter) resetMatchBuffer() {
	s.muxi = 0        // multiplexer index
	s.lastAlpha = 0.0 // crossfade coefficient
	s.winner = -1
	s.lastWinner = -1
	s.setLoDataLoc(0)
	s.setHiDataLoc(0)
	s.lastShingleSize = -1
	if s.matcher != nil {
		s.matcher.clearFrameQueue()
	}
	s.normsNeedUpdate = true
}

func (s *soundSpotter) syncOnShingleStart() {
	if s.muxi == 0 { // parameters and statistics must only change on match boundaries
		s.LoK = s.ifaceLoK
		s.HiK = s.ifaceHiK
		s.minASB = s.ifaceLoFeature
		s.maxASB = s.ifaceHiFeature
		s.shingleSize = s.ifaceShingleSize
		s.normsNeedUpdate = s.normsNeedUpdate || !(s.lastLoFeature == s.minASB &&
			s.lastHiFeature == s.maxASB &&
			s.lastShingleSize == s.shingleSize &&
			s.lastLoK == s.LoK &&
			s.lastHiK == s.HiK)
		// update database statistics based on current search parameters
		if s.normsNeedUpdate {
			// update the power threshold data
			if s.shingleSize != s.lastShingleSize {
				s.sourcePowersCurrent.copy(s.sourcePowers)
				seriesMean(s.sourcePowersCurrent.series, idxT(s.shingleSize), idxT(s.getLengthSourceShingles()))
				s.lastShingleSize = s.shingleSize
				s.matcher.clearFrameQueue()
			}
			// update the database norms for new parameters
			s.matcher.updateDatabaseNorms(s.sourceShingles, s.shingleSize, s.getLengthSourceShingles(),
				s.minASB, s.maxASB, s.LoK, s.HiK)
			// set the change indicators
			s.normsNeedUpdate = false
			s.lastLoFeature = s.minASB
			s.lastHiFeature = s.maxASB
			s.lastLoK = s.LoK
			s.lastHiK = s.HiK
		}
	}
}

func (s *soundSpotter) zeroBuf(buf ss_sample, length uint64) {
	i := 0
	for ; length > 0; length-- {
		buf[i] = 0.0
		i++
	}
}
