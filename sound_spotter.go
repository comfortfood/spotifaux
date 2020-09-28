package spotifaux

import "C"

const SS_MAX_DATABASE_SECS = 7200
const SS_FFT_LENGTH = 800
const WindowLength = 400
const Hop = 160
const SAMPLE_RATE = 16000

type soundSpotter struct {
	sampleRate int
	InShingles [][]float64

	maxDBSize int // Maximum number of source frames to extract (initial size of database)

	ChosenFeatures []int

	ShingleSize int

	CqtN int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func NewSoundSpotter(sampleRate int, cqtN int) *soundSpotter {

	s := &soundSpotter{
		ChosenFeatures: []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		ShingleSize:    25,
		CqtN:           cqtN,
	}

	s.maxDBSize = (int)((float32(sampleRate) / float32(WindowLength)) * SS_MAX_DATABASE_SECS)
	s.InShingles = make([][]float64, s.ShingleSize)
	for i := 0; i < s.ShingleSize; i++ {
		s.InShingles[i] = make([]float64, s.CqtN)
	}

	return s
}

//func (s *soundSpotter) Output(inPower float64, winner int) []float64 {
//	outputLength := Hop * s.ShingleSize
//	outputBuffer := make([]float64, outputLength) // fix size at constructor ?
//	if winner > -1 {
//		// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
//		// sqrt(env2) has already been calculated, only take sqrt(env1) here
//		envFollow := 1.0
//		alpha := envFollow*math.Sqrt(inPower/s.dbPowers[winner]) + (1 - envFollow)
//		for p := 0; p < outputLength && winner*Hop+p < len(s.dbBuf); p++ {
//			output := alpha * s.dbBuf[winner*Hop+p]
//			if output > 1.12 {
//				output = 1.12
//			} else if output < -1.12 {
//				output = -1.12
//			}
//			outputBuffer[p] = output * 0.8
//		}
//	}
//	return outputBuffer
//}
