package spotifaux

import "errors"

type wavSource struct {
	dbBuf []float64
	i     int
}

func NewWavSource() *wavSource {
	sf, err := NewSoundFile("/Users/wyatttall/git/spotifaux/recreate/westminster-16kHz.wav")
	if err != nil {
		panic(err)
	}
	defer sf.Close()

	dbBuf := make([]float64, sf.Frames)
	_, err = sf.ReadFrames(dbBuf)
	if err != nil {
		panic(err)
	}

	fftN := SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1 // linear frequency power spectrum values (automatic)

	e := NewFeatureExtractor(SAMPLE_RATE, fftN, fftOutN)

	NewSoundSpotter(SAMPLE_RATE, dbBuf, sf.Frames, e.CqtN)

	return &wavSource{dbBuf: dbBuf}
}

func (s *wavSource) Float64() (float64, error) {
	i := s.i
	if i >= len(s.dbBuf) {
		return 0, errors.New("out of bounds")
	}
	s.i += 1
	return s.dbBuf[i], nil
}

func (s *wavSource) Close() error {
	return nil
}
