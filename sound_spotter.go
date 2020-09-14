package main

import "C"
import (
	"fmt"
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_MAX_SHINGLE_SZ = 32
const SS_FFT_LENGTH = 4096

type SoundSpotterStatus int

const (
	STOP SoundSpotterStatus = iota
	EXTRACTANDSPOT
	EXTRACT
	SPOT
	THRU
	DUMP
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

	MAX_Queued int // maximum length of segment priority queue
	maxF       int // Maximum number of source frames to extract (initial size of database)

	minASB          int
	maxASB          int
	NASB            int
	lastLoFeature   int // persist feature parameters for database norming
	lastHiFeature   int // persist feature parameters for database norming
	ifaceLoFeature  int // store interface values to change on shingle boundary
	ifaceHiFeature  int
	normsNeedUpdate bool // flag to indicate that database norms are dirty
	isMaster        int  // flag to indicate master soundspotter, extracts features
	audioDisabled   int  // whether to process audio
	basisWidth      int
	x               ss_sample // SoundSpotter pointer to PD internal buf
	bufLen          int64

	audioOutputBuffer ss_sample // Matchers buffer
	muxi              int       // SHINGLEing overlap-add buffer multiplexer index
	lastAlpha         float64   // envelope follow of previous output
	hammingWin2       ss_sample

	queueSize        int
	shingleSize      int
	lastShingleSize  int
	ifaceShingleSize int

	shingleHop       int
	xPtr             int64
	XPtr             int
	lastWinner       int      // previous winning frame
	winner           int      // winning frame/shingle in seriesOfVectors match
	matcher          *matcher // shingle matching algorithm
	featureExtractor *featureExtractor

	pwr_abs_thresh float64 // don't match below this threshold
	pwr_rel_thresh float64 // don't match below this threshold
	Radius         float64 // Search Radius

	LoK      int // Database start point marker
	HiK      int // Database end point marker
	lastLoK  int
	lastHiK  int
	ifaceLoK int
	ifaceHiK int

	envFollow     float64 // amount to follow the input audio's energy envelope
	betaParameter float32 // parameter for probability function (-1.0f * beta)

	soundSpotterStatus SoundSpotterStatus
	SS_DEBUG           bool
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
		isMaster:           1,
		lastShingleSize:    -1,
		ifaceShingleSize:   4,
		shingleHop:         1,
		lastWinner:         -1,
		winner:             -1,
		pwr_abs_thresh:     0.000001,
		pwr_rel_thresh:     0.1,
		Radius:             0.0,
		LoK:                -1,
		HiK:                -1,
		lastLoK:            -1,
		lastHiK:            -1,
		betaParameter:      1.0,
		soundSpotterStatus: STOP,
		//minASB:             5,
		//queueSize:          10,
		//envFollow:          0,
	}

	DEBUGINFO("feature extractor...")
	ss.featureExtractor = &featureExtractor{sampleRate: sampleRate, WindowLength: WindowLength, fftN: SS_FFT_LENGTH}
	ss.featureExtractor.initializeFeatureExtractor()
	ss.NASB = ss.featureExtractor.dctN
	DEBUGINFO("maxF...")
	ss.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	DEBUGINFO("makeHammingWin2...")
	ss.makeHammingWin2()
	fmt.Printf("inShingle...")
	ss.MAX_SHINGLE_SIZE = SS_MAX_SHINGLE_SZ
	ss.inShingle = NewSeriesOfVectors(idxT(ss.NASB), idxT(ss.MAX_SHINGLE_SIZE))
	ss.inPowers = NewSeriesOfVectors(idxT(ss.MAX_SHINGLE_SIZE), idxT(1))

	DEBUGINFO("audioOutputBuffer...")
	ss.audioOutputBuffer = make(ss_sample, WindowLength*ss.MAX_SHINGLE_SIZE*numChannels) // fix size at constructor ?
	ss.resetShingles(ss.maxF)                                                            // Some default number of shingles
	return ss
}

func (s *soundSpotter) getStatus() SoundSpotterStatus {
	return s.soundSpotterStatus
}

// FIXME: source sample buffer (possibly externally allocated)
// Require different semantics for externally and internally
// allocated audio buffers
func (s *soundSpotter) getAudioDatabaseBuf() ss_sample {
	return s.x
}

// samples not frames (i.e. includes channels)
func (s *soundSpotter) getAudioDatabaseBufLen() int64 {
	return s.bufLen
}

// samples not frames (i.e. includes channels)
func (s *soundSpotter) getAudioDatabaseFrames() int64 {
	return s.bufLen / int64(s.numChannels)
}

func (s *soundSpotter) getAudioDatabaseNumChannels() int {
	return s.numChannels
}

func (s *soundSpotter) getLengthSourceShingles() int {
	return int(math.Ceil(float64(s.bufLen) / (float64(s.numChannels) * float64(s.WindowLength))))
}

func (s *soundSpotter) getXPtr() int {
	return s.XPtr
}

func (s *soundSpotter) getxPtr() int64 {
	return s.xPtr
}

func (s *soundSpotter) getMaxF() int {
	return s.maxF
}

// Implement triggers based on state transitions
// Status changes to EXTRACT or EXTRACTANDSPOT
// require memory checking and possible allocation
//
// FIXME: Memory allocation discipline is suspicious
// Sometimes the audio buffer is user allocated, other times it is auto allocated
// EXTRACT: requires a source buffer (x) to be provided, allocates database
// EXTRACTANDSPOT: allocates its own source buffer and database
func (s *soundSpotter) setStatus(stat SoundSpotterStatus) int {

	if stat == s.soundSpotterStatus {
		fmt.Printf("failed no new state\n")
		return 0 // fail if no new state
	}

	switch stat {

	case SPOT:
		break

	case EXTRACT:
	case EXTRACTANDSPOT:
		if s.SS_DEBUG {
			fmt.Printf("extract : bufLen=%d c=%d f=%d\n", s.getAudioDatabaseBufLen(), s.getAudioDatabaseNumChannels(), s.getAudioDatabaseFrames())
		}
		if stat == EXTRACT && s.getAudioDatabaseBuf() == nil {
			fmt.Printf("failed no audio\n")
			return 0 // fail if no audio to extract (user allocated audio buffer)
		}
		if stat == EXTRACTANDSPOT && s.getAudioDatabaseBuf() == nil {
			s.setAudioDatabaseBuf(make(ss_sample, s.maxF*s.WindowLength), int64(s.maxF*s.WindowLength), 1) // allocate audio buffer
		}
		break

	case STOP:
	case THRU:
	case DUMP:
		break

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
	if s.SS_DEBUG {
		fmt.Printf("setAudioDatabaseBuf : len=%d c=%d\n", l, channels)
	}

	s.numChannels = channels
	s.x = b
	l = l * int64(channels)
	if l > int64(s.maxF)*int64(s.WindowLength)*int64(channels) {
		l = int64(s.maxF) * int64(s.WindowLength) * int64(channels)
	}
	s.bufLen = l
	s.xPtr = 0
}

// Buffer reset for EXTRACT and EXTRACTANDSPOT
func (s *soundSpotter) resetBufPtrs() {
	s.XPtr = 0
	s.xPtr = 0
	if s.SS_DEBUG {
		fmt.Printf("resetBufPtrs : XPtr=0, xptr=0\n")
	}
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

func (s *soundSpotter) run(n int, ins1, ins2, outs1, outs2 ss_sample) {
	var m int
	if s.SS_DEBUG {
		fmt.Printf(" run : status=%d\n", s.soundSpotterStatus)
	}

	switch s.soundSpotterStatus {
	case STOP:
		i := 0
		for ; n > 0; n-- {
			outs2[i] = 0
			i++
		}
		return

	case THRU:
		m = 0
		for m < s.WindowLength {
			outs2[m] = ins2[m]
			m++
		}
		return

	case EXTRACT:
		DEBUGINFO("featureExtractor...")
		s.resetShingles(s.getLengthSourceShingles())
		s.resetMatchBuffer()
		s.XPtr = s.featureExtractor.extractSeriesOfVectors(s.getAudioDatabaseBuf(),
			s.numChannels, s.getAudioDatabaseBufLen(),
			s.sourceShingles.series, s.sourcePowers.series,
			s.sourceShingles.rows, s.getLengthSourceShingles())
		s.xPtr = s.getAudioDatabaseBufLen()
		s.normsNeedUpdate = true
		if s.SS_DEBUG {
			fmt.Printf(" run(EXTRACT) : xPtr=%d, XPtr=%d\n", s.xPtr, s.XPtr)
		}
		s.setStatus(SPOT)
		return

	case SPOT:
		s.spot(n, ins1, ins2, outs1, outs2)
		return

	case EXTRACTANDSPOT:
		//s.liveSpot(n, ins1, ins2, outs1, outs2)
		return

	default:
	}
}

func (s *soundSpotter) spot(n int, ins1, ins2, outs1, outs2 ss_sample) {
	if s.checkExtracted() == 0 {
		return // features extracted?
	}
	if s.muxi == 0 {
		s.synchOnShingleStart() // update parameters at shingleStart
	}
	// ins2 holds the audio samples, convert ins2 to outs1 (FFT buffer)
	s.featureExtractor.extractVector(n, ins1, ins2, outs1, &s.inPowers.series[s.muxi], s.isMaster)
	// insert MFCC into SeriesOfVectors
	s.inShingle.insert(outs1, idxT(s.muxi))
	// insert shingles into Matcher
	s.matcher.insert(s.inShingle, s.shingleSize, s.sourceShingles, s.XPtr, s.muxi, s.minASB, s.maxASB, s.LoK, s.HiK)
	// post-insert buffer multiplex increment
	s.muxi = s.incrementMultiplexer(s.muxi, s.shingleSize)
	// Do the matching at shingle end
	if s.muxi == 0 {
		s.match()
	}
	// generate current frame's output sample and update everything
	s.updateAudioOutputBuffers(outs2)
}

func (s *soundSpotter) incrementMultiplexer(multiplex, sz int) int {
	return (multiplex + 1) % sz
}

func (s *soundSpotter) checkExtracted() int {
	if s.XPtr == 0 || s.getAudioDatabaseBufLen() == 0 {
		fmt.Printf("You must extract some features from source material first! XPtr=%d,xPtr=%du", s.XPtr, s.getAudioDatabaseBufLen())
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
	if s.SS_DEBUG {
		fmt.Printf(" match : lastWinner=%d, winner=%d, inPwMn=%f\n", s.lastWinner, s.winner, inPwMn)
	}
	if inPwMn > s.pwr_abs_thresh {
		s.lastWinner = s.winner // preserve previous state for cross-fading audio output
		// matched filter matching to get winning database shingle
		s.winner = s.matcher.match(s.Radius, s.shingleSize, s.XPtr, s.LoK, s.HiK, s.queueSize,
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
func (s *soundSpotter) updateAudioOutputBuffers(outSamps ss_sample) {
	p2 := s.muxi * s.WindowLength * s.numChannels // so that this is synchronized on match boundaries
	p1 := 0
	nn := s.WindowLength * s.numChannels
	for ; nn > 0; nn-- {
		outSamps[p1] = s.audioOutputBuffer[p2] // multi-channel output
		p1++
		p2++
	}
}

func (s *soundSpotter) reportResult() int {
	return s.winner
}

// sampleBuf() copy n*shingleHop to audioOutputBuffer from best matching segment in source buffer x[]
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
			s.audioOutputBuffer[p] = alpha * s.x[q]
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
					s.audioOutputBuffer[p] = alpha*s.x[p1]*s.hammingWin2[w1] + s.lastAlpha*s.x[q]*s.hammingWin2[w2]
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
	DEBUGINFO("resetShingles()...")

	if newSize > s.maxF {
		newSize = s.maxF
	}

	if s.sourceShingles == nil {
		DEBUGINFO("allocate new sourceShingles...")
		s.sourceShingles = NewSeriesOfVectors(idxT(s.NASB), idxT(newSize))
		s.sourcePowers = NewSeriesOfVectors(idxT(newSize), idxT(1))
		s.sourcePowersCurrent = NewSeriesOfVectors(idxT(newSize), idxT(1))
		DEBUGINFO("allocate matcher...")
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
	if s.SS_DEBUG {
		fmt.Printf("len: %d\n", s.getAudioDatabaseBufLen())
		fmt.Printf("frm: %d\n", s.getAudioDatabaseFrames())
		fmt.Printf("dbSz: %d\n", s.getLengthSourceShingles())
		fmt.Printf("stat: %d\n", s.getStatus())
		fmt.Printf("quSz: %d\n", s.queueSize)
		fmt.Printf("shSz: %d\n", s.shingleSize)
	}
}

func (s *soundSpotter) synchOnShingleStart() {
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
