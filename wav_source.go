package main

import (
	"fmt"
)

type wavSource struct {
	soundBuf ss_sample
	i        int
}

func newWavSource() *wavSource {
	SS := newSoundSpotter(44100, N, 2)
	SF := &soundFile{}
	retval := SF.sfOpen("/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav")
	if retval < 0 {
		fmt.Printf("Could not open %s\n", "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav")
	}
	SS.setAudioDatabaseBuf(SF.soundBuf, SF.numFrames, int(SF.info.Channels))

	return &wavSource{soundBuf: SF.soundBuf}
}

func (s *wavSource) Float64() float64 {
	i := s.i
	s.i += 1
	return s.soundBuf[i]
}

func (s *wavSource) Close() error {
	return nil
}
