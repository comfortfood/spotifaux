package spotifaux

type wavSource struct {
	dbBuf []float64
	i     int
}

func NewWavSource() *wavSource {
	sf, err := NewSoundFile("/Users/wyatttall/git/spotifaux/recreate/bell-16kHz.wav")
	if err != nil {
		panic(err)
	}
	defer sf.Close()

	dbBuf := make([]float64, sf.Frames*int64(sf.Channels))
	_, err = sf.ReadFrames(dbBuf)
	if err != nil {
		panic(err)
	}

	fftN := SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1 // linear frequency power spectrum values (automatic)

	e := NewFeatureExtractor(SAMPLE_RATE, fftN, fftOutN)

	NewSoundSpotter(SAMPLE_RATE, sf.Channels, dbBuf, sf.Frames, e.CqtN)

	return &wavSource{dbBuf: dbBuf}
}

func (s *wavSource) Float64() float64 {
	i := s.i
	s.i += 1
	return s.dbBuf[i]
}

func (s *wavSource) Close() error {
	return nil
}
