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

	audioDatabaseBuf := make([]float64, sf.frames*int64(sf.channels))
	_, err = sf.ReadFrames(audioDatabaseBuf)
	if err != nil {
		panic(err)
	}

	newSoundSpotter(44100, N, sf.channels, audioDatabaseBuf, sf.frames)

	return &wavSource{audioDatabaseBuf: audioDatabaseBuf}
}

func (s *wavSource) Float64() float64 {
	i := s.i
	s.i += 1
	return s.audioDatabaseBuf[i]
}

func (s *wavSource) Close() error {
	return nil
}
