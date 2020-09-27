package spotifaux

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_FFT_LENGTH = 800
const WindowLength = 400
const Hop = 160
const SAMPLE_RATE = 16000

type soundSpotter struct {
	sampleRate int
	dbShingles [][]float64
	InShingles [][]float64
	dbPowers   []float64
	InPowers   []float64

	maxDBSize int // Maximum number of source frames to extract (initial size of database)

	ChosenFeatures       []int
	dbBuf                []float64 // SoundSpotter pointer to PD internal buf
	LengthSourceShingles int

	ShingleSize int

	Matcher *matcher // shingle matching algorithm

	pwr_abs_thresh float64 // don't match below this threshold

	CqtN int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func NewSoundSpotter(sampleRate int, dbBuf []float64, numFrames int64, cqtN int) *soundSpotter {

	s := &soundSpotter{
		ChosenFeatures: []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21},
		ShingleSize:    50,
		pwr_abs_thresh: 0.000001,
		CqtN:           cqtN,
	}

	s.maxDBSize = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.InShingles = make([][]float64, s.ShingleSize)
	for i := 0; i < s.ShingleSize; i++ {
		s.InShingles[i] = make([]float64, s.CqtN)
	}
	s.InPowers = make([]float64, s.ShingleSize+1)

	s.dbShingles = make([][]float64, s.maxDBSize)
	for i := 0; i < s.maxDBSize; i++ {
		s.dbShingles[i] = make([]float64, s.CqtN)
	}
	s.dbPowers = make([]float64, s.maxDBSize)

	s.dbBuf = dbBuf
	s.LengthSourceShingles = int(math.Ceil(float64(numFrames) / (float64(Hop))))

	// Cross-correlation matrix
	D := make([][]float64, s.ShingleSize)
	for i := 0; i < s.ShingleSize; i++ {
		D[i] = make([]float64, s.LengthSourceShingles+s.ShingleSize-1)
	}

	matcher := &matcher{
		D: D,
	}
	matcher.frameQueue.Init()
	s.Matcher = matcher

	return s
}

// Perform matching on shingle boundary
func (s *soundSpotter) Match(inPower, qN0 float64, sNorm []float64) int {
	if inPower > s.pwr_abs_thresh {
		// matched filter matching to get winning database shingle
		return s.Matcher.match(s, qN0, sNorm)
	}
	return -1
}

func (s *soundSpotter) Output(inPower float64, winner int) []float64 {
	outputLength := Hop * s.ShingleSize
	outputBuffer := make([]float64, outputLength) // fix size at constructor ?
	if winner > -1 {
		// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
		// sqrt(env2) has already been calculated, only take sqrt(env1) here
		envFollow := 1.0
		alpha := envFollow*math.Sqrt(inPower/s.dbPowers[winner]) + (1 - envFollow)
		for p := 0; p < outputLength && winner*Hop+p < len(s.dbBuf); p++ {
			output := alpha * s.dbBuf[winner*Hop+p]
			if output > 1.12 {
				output = 1.12
			} else if output < -1.12 {
				output = -1.12
			}
			outputBuffer[p] = output * 0.8
		}
	}
	return outputBuffer
}

func (s *soundSpotter) SyncOnShingleStart() {
	// update the power threshold data
	s.Matcher.frameQueue.Init()
}
