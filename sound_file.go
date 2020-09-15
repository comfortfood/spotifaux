package main

import (
	"errors"
	"github.com/mkb218/gosndfile/sndfile"
)

const SF_MAX_NUM_FRAMES = 7200 * 44100

type soundFile struct {
	channels int
	file     *sndfile.File
	frames   int64
}

// Attempt to open sound file
// return -1 if cannot open soundFile
// else return number of frames read
func newSoundFile(fileName string) (*soundFile, error) {
	sf := &soundFile{}

	// Open sound file
	info := &sndfile.Info{}
	file, err := sndfile.Open(fileName, sndfile.Read, info)
	if err != nil {
		return nil, err
	}
	sf.file = file

	if !sndfile.FormatCheck(*info) {
		panic(errors.New("bad format"))
	}
	sf.channels = int(info.Channels)
	sf.frames = info.Frames
	if sf.frames > SF_MAX_NUM_FRAMES {
		sf.frames = SF_MAX_NUM_FRAMES
	}
	return sf, nil
}

func (f *soundFile) ReadFrames(out interface{}) (read int64, err error) {
	return f.file.ReadFrames(out)
}

func (f *soundFile) Close() error {
	return f.file.Close()
}
