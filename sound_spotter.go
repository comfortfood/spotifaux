package spotifaux

import "C"
import "math"

const SS_FFT_LENGTH = 800
const WindowLength = 400
const Hop = 160
const SAMPLE_RATE = 16000

type soundSpotter struct {
	sampleRate int
	InShingles [][]float64

	ChosenFeatures []int

	ShingleSize int

	CqtN int // number of constant-Q coefficients (automatic)
}

// multi-channel soundspotter
// expects MONO input and possibly multi-channel DATABASE audio output
func NewSoundSpotter(cqtN int) *soundSpotter {

	s := &soundSpotter{
		ChosenFeatures: []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		ShingleSize:    25,
		CqtN:           cqtN,
	}

	s.InShingles = make([][]float64, s.ShingleSize)
	for i := 0; i < s.ShingleSize; i++ {
		s.InShingles[i] = make([]float64, s.CqtN)
	}

	return s
}

func (s *soundSpotter) Output(sf *soundFile, winner int, inPower float64) ([]float64, error) {

	outputLength := Hop * s.ShingleSize
	outputBuffer := make([]float64, outputLength) // fix size at constructor ?
	if winner > -1 {

		_, err := sf.Seek(int64(winner * Hop))
		if err != nil {
			return nil, err
		}

		buf := make([]float64, outputLength)
		_, err = sf.ReadFrames(buf)
		if err != nil {
			return nil, err
		}

		dbPower := 0.0
		for _, val := range buf {
			dbPower += math.Pow(val, 2)
		}
		dbPower /= float64(len(buf))

		// Envelope follow factor is alpha * sqrt(env1/env2) + (1-alpha)
		// sqrt(env2) has already been calculated, only take sqrt(env1) here
		envFollow := 1.0
		alpha := envFollow*math.Sqrt(inPower/dbPower) + (1.0 - envFollow)
		for p := 0; p < outputLength; p++ {
			output := alpha * buf[p]
			if output > 1.12 {
				output = 1.12
			} else if output < -1.12 {
				output = -1.12
			}
			outputBuffer[p] = output * 0.8
		}
	}
	return outputBuffer, nil
}
