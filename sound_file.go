package main

import (
	"errors"
	"github.com/mkb218/gosndfile/sndfile"
)

const SF_MAX_NUM_FRAMES = 7200 * 44100

type soundFile struct {
	soundBuf  ss_sample
	numFrames int64
	inFile    *sndfile.File
	info      *sndfile.Info
}

// Attempt to open sound file
// return -1 if cannot open soundFile
// else return number of frames read
func newSoundFile(inFileName string) (*soundFile, error) {
	f := &soundFile{}

	// Open sound file
	f.info = &sndfile.Info{}
	inFile, err := sndfile.Open(inFileName, sndfile.Read, f.info)
	if err != nil {
		return nil, err
	}
	f.inFile = inFile

	if !sndfile.FormatCheck(*f.info) {
		panic(errors.New("bad format"))
	}
	f.numFrames = f.info.Frames
	if f.numFrames > SF_MAX_NUM_FRAMES {
		f.numFrames = SF_MAX_NUM_FRAMES
	}
	f.soundBuf = make(ss_sample, f.numFrames*int64(f.info.Channels))
	_, err = f.inFile.ReadFrames(f.soundBuf)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (f *soundFile) Close() error {
	return f.inFile.Close()
}
