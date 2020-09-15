package main

type wavSource struct {
	soundBuf ss_sample
	i        int
}

func newWavSource() *wavSource {
	sf, err := newSoundFile("/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav")
	if err != nil {
		panic(err)
	}
	newSoundSpotter(44100, N, 2, sf.soundBuf, sf.numFrames, int(sf.info.Channels))

	return &wavSource{soundBuf: sf.soundBuf}
}

func (s *wavSource) Float64() float64 {
	i := s.i
	s.i += 1
	return s.soundBuf[i]
}

func (s *wavSource) Close() error {
	return nil
}
