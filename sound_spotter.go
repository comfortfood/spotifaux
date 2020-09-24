package spotifaux

import "C"
import (
	"math"
)

const SS_MAX_DATABASE_SECS = 7200
const SS_FFT_LENGTH = 800
const WindowLength = 400
const Hop = 400
const SAMPLE_RATE = 16000

type soundSpotter struct {
	sampleRate int
	dbShingles [][]float64
	InShingles [][]float64
	dbPowers   []float64
	InPowers   []float64

	maxF int // Maximum number of source frames to extract (initial size of database)

	loFeature            int
	hiFeature            int
	dbBuf                []float64 // SoundSpotter pointer to PD internal buf
	lengthSourceShingles int

	ShingleSize int

	dbSize  int
	Winner  int      // winning frame/shingle in seriesOfVectors match
	Matcher *matcher // shingle matching algorithm

	pwr_abs_thresh float64 // don't match below this threshold

	CqtN int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func NewSoundSpotter(sampleRate int, dbBuf []float64, numFrames int64, cqtN int) *soundSpotter {

	s := &soundSpotter{
		sampleRate:     sampleRate,
		loFeature:      3,
		hiFeature:      20,
		ShingleSize:    4,
		Winner:         -1,
		pwr_abs_thresh: 0.000001,
		dbSize:         0,
		CqtN:           cqtN,
	}

	s.maxF = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.InShingles = make([][]float64, s.ShingleSize)
	for i := 0; i < s.ShingleSize; i++ {
		s.InShingles[i] = make([]float64, s.CqtN)
	}
	s.InPowers = make([]float64, s.ShingleSize+1)

	s.dbShingles = make([][]float64, s.maxF)
	for i := 0; i < s.maxF; i++ {
		s.dbShingles[i] = make([]float64, s.CqtN)
	}
	s.dbPowers = make([]float64, s.maxF)
	s.Matcher = &matcher{}
	s.Matcher.resize(s.ShingleSize, s.maxF)

	s.dbBuf = dbBuf
	s.lengthSourceShingles = int(math.Ceil(float64(numFrames) / (float64(Hop))))
	s.Matcher.frameQueue.Init()

	return s
}

// Perform matching on shingle boundary
func (s *soundSpotter) Match() []float64 {
	outputLength := Hop*(s.ShingleSize-1) + WindowLength
	outputBuffer := make([]float64, outputLength) // fix size at constructor ?
	// calculate powers for detecting silence and balancing output with input
	SeriesMean(s.InPowers, s.ShingleSize)
	if s.InPowers[0] > s.pwr_abs_thresh {
		// matched filter matching to get winning database shingle
		s.Winner = s.Matcher.match(s)
		if s.Winner > -1 {
			// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
			// sqrt(env2) has already been calculated, only take sqrt(env1) here
			envFollow := 0.5
			alpha := envFollow*math.Sqrt(s.InPowers[0]/s.dbPowers[s.Winner]) + (1 - envFollow)
			if s.Winner > -1 {
				for p := 0; p < outputLength && s.Winner*Hop+p < len(s.dbBuf); p++ {
					output := alpha * s.dbBuf[s.Winner*Hop+p]
					if output > 1.12 {
						output = 1.12
					} else if output < -1.12 {
						output = -1.12
					}
					outputBuffer[p] = output * 0.8
				}
			}
		}
	}
	return outputBuffer
}

func (s *soundSpotter) SyncOnShingleStart() {

	// update the power threshold data
	s.Matcher.frameQueue.Init()

	// update the database norms for new parameters
	s.Matcher.updateDatabaseNorms(s)
}
