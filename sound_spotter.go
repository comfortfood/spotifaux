package main

import "C"
import (
	"fmt"
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_MAX_SHINGLE_SZ = 32
const SS_MAX_QUEUE_SZ = 10000
const SS_NUM_BASIS = 83
const SS_MAX_RADIUS = 4
const SS_FFT_LENGTH = 4096
const SS_WINDOW_LENGTH = 2048

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
	normsNeedUpdate int // flag to indicate that database norms are dirty
	isMaster        int // flag to indicate master soundspotter, extracts features
	audioDisabled   int // whether to process audio
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

	pwr_abs_thresh ss_sample // don't match below this threshold
	pwr_rel_thresh ss_sample // don't match below this threshold
	Radius         ss_sample // Search Radius

	LoK      int // Database start point marker
	HiK      int // Database end point marker
	lastLoK  int
	lastHiK  int
	ifaceLoK int
	ifaceHiK int

	envFollow     ss_sample // amount to follow the input audio's energy envelope
	betaParameter float32   // parameter for probability function (-1.0f * beta)

	soundSpotterStatus SoundSpotterStatus
	SS_DEBUG           bool
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func newSoundSpotter(sampleRate int, WindowLength int, numChannels int) *soundSpotter {
	ss := &soundSpotter{}

	DEBUGINFO("feature extractor...")
	ss.featureExtractor = &featureExtractor{sampleRate: sampleRate, WindowLength: WindowLength, fftN: SS_FFT_LENGTH}
	ss.featureExtractor.initializeFeatureExtractor()
	NASB := ss.featureExtractor.dctN
	DEBUGINFO("maxF...")
	ss.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	DEBUGINFO("makeHammingWin2...")
	ss.makeHammingWin2()
	fmt.Printf("inShingle...")
	MAX_SHINGLE_SIZE := SS_MAX_SHINGLE_SZ
	ss.inShingle = &seriesOfVectors{M: idxT(NASB), N: idxT(MAX_SHINGLE_SIZE)}
	ss.inPowers = &seriesOfVectors{M: idxT(MAX_SHINGLE_SIZE), N: idxT(1)}

	DEBUGINFO("audioOutputBuffer...")
	ss.audioOutputBuffer = make(ss_sample, WindowLength*MAX_SHINGLE_SIZE*numChannels) // fix size at constructor ?
	ss.resetShingles(ss.maxF)                                                         // Some default number of shingles
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
	//	numCols := s.sourceShingles.getCols()
	//	numRows := s.sourceShingles.getRows()
	//	k := 0
	//
	//	// zero-out source shingles
	//	for k = 0; k < numCols; k++ {
	//		s.zeroBuf(s.sourceShingles.getCol(k), numRows)
	//	}
	//	numCols = s.inShingle.getCols()
	//	// zero-out inShingle
	//	for k = 0; k < numCols; k++ {
	//		s.zeroBuf(s.inShingle.getCol(k), numRows)
	//	}
	//
	//	// zero-out power sequences
	//	s.zeroBuf(s.inPowers.getCol(0), s.inPowers.getRows())
	//	s.zeroBuf(s.sourcePowers.getCol(0), s.sourcePowers.getRows())
	//}
	//
	//// audio output buffer
	//s.zeroBuf(s.audioOutputBuffer, SS_MAX_SHINGLE_SZ*s.WindowLength*s.numChannels)
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
		for ; n > 0; n-- {
			outs2[n] = 0
		}
		return

	case THRU:
		m = 0
		for ; m < s.WindowLength; {
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
			s.sourceShingles.getSeries(), s.sourcePowers.getSeries(),
			s.sourceShingles.getRows(), s.getLengthSourceShingles())
		s.xPtr = s.getAudioDatabaseBufLen()
		s.normsNeedUpdate = 1
		if s.SS_DEBUG {
			fmt.Printf(" run(EXTRACT) : xPtr=%d, XPtr=%d\n", s.xPtr, s.XPtr)
		}
		s.setStatus(SPOT)
		return

	case SPOT:
		//s.spot(n, ins1, ins2, outs1, outs2)
		return

	case EXTRACTANDSPOT:
		//s.liveSpot(n, ins1, ins2, outs1, outs2)
		return

	default:
	}
}

func (s *soundSpotter) resetShingles(newSize int) int {
	DEBUGINFO("resetShingles()...")

	if newSize > s.maxF {
		newSize = s.maxF
	}

	if s.sourceShingles == nil {
		DEBUGINFO("allocate new sourceShingles...")
		s.sourceShingles = &seriesOfVectors{M: idxT(s.NASB), N: idxT(newSize)}
		s.sourcePowers = &seriesOfVectors{M: idxT(newSize), N: idxT(1)}
		s.sourcePowersCurrent = &seriesOfVectors{M: idxT(newSize), N: idxT(1)}
		DEBUGINFO("allocate matcher...")
		MAX_SHINGLE_SIZE := SS_MAX_SHINGLE_SZ
		s.matcher = &matcher{maxShingleSize: MAX_SHINGLE_SIZE, maxDBSize: newSize}
	}

	s.resetBufPtrs() // fill shingles with zeros and set buffer pointers to zero
	return newSize
}

// perform reset on new soundfile load or extract
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
	s.normsNeedUpdate = 1
	if s.SS_DEBUG {
		fmt.Printf("len: %d\n", s.getAudioDatabaseBufLen())
		fmt.Printf("frm: %d\n", s.getAudioDatabaseFrames())
		fmt.Printf("dbSz: %d\n", s.getLengthSourceShingles())
		fmt.Printf("stat: %d\n", s.getStatus())
		fmt.Printf("quSz: %d\n", s.queueSize)
		fmt.Printf("shSz: %d\n", s.shingleSize)
	}
}
