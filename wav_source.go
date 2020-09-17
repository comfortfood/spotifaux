package main

type wavSource struct {
	audioDatabaseBuf ss_sample
	i                int
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

	e := newFeatureExtractor(44100, WindowLength, SS_FFT_LENGTH)

	newSoundSpotter(44100, WindowLength, sf.channels, dbBuf, sf.frames, e.cqtN)

	return &wavSource{audioDatabaseBuf: dbBuf}
}

func (s *wavSource) Float64() float64 {
	i := s.i
	s.i += 1
	return s.audioDatabaseBuf[i]
}

func (s *wavSource) Close() error {
	return nil
}
