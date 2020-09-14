package main

import (
	"errors"
	"fmt"
)

type wavSource struct {
	soundBuf ss_sample
	i        int
}

func newWavSource() *wavSource {
	sf := &soundFile{}
	retval := sf.sfOpen("/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav")
	if retval < 0 {
		panic(errors.New(fmt.Sprintf("Could not open %s", "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav")))
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
