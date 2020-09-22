package main

type wavSource struct {
	dbBuf []float64
	i     int
}

func newWavSource() *wavSource {
	sf, err := newSoundFile("/Users/wyatttall/git/spotifaux/Madonna.wav")
	if err != nil {
		panic(err)
	}
	defer sf.Close()

	dbBuf := make([]float64, sf.frames*int64(sf.channels))
	_, err = sf.ReadFrames(dbBuf)
	if err != nil {
		panic(err)
	}

	fftN := SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1 // linear frequency power spectrum values (automatic)

	e := newFeatureExtractor(44100, WindowLength, fftN, fftOutN)

	newSoundSpotter(44100, WindowLength, sf.channels, dbBuf, sf.frames, e.cqtN)

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
