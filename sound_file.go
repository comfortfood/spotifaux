package main

import (
	"github.com/mkb218/gosndfile/sndfile"
)

const SF_MAX_NUM_FRAMES = 7200 * 44100

type soundFile struct {
	soundBuf  ss_sample
	numFrames int64
	inFile    *sndfile.File
	info      *sndfile.Info
}

func (f *soundFile) sfCleanUp() {
	if f.inFile != nil {
		f.soundBuf = nil
		defer f.inFile.Close()
		f.inFile = nil
	}
}

// Attempt to open sound file
// return -1 if cannot open soundFile
// else return number of frames read
func (f *soundFile) sfOpen(inFileName string) int64 {
	f.sfCleanUp()

	// Open sound file
	f.info = &sndfile.Info{}
	inFile, err := sndfile.Open(inFileName, sndfile.Read, f.info)
	if err != nil {
		return -1
	}
	f.inFile = inFile

	if !sndfile.FormatCheck(*f.info) {
		return -1
	}
	f.numFrames = f.info.Frames
	if f.numFrames > SF_MAX_NUM_FRAMES {
		f.numFrames = SF_MAX_NUM_FRAMES
	}
	result := f.loadSound()
	return result
}

// Read the entire soundfile into a new float buffer
func (f *soundFile) loadSound() int64 {
	f.soundBuf = make(ss_sample, f.numFrames*int64(f.info.Channels))
	numRead, err := f.inFile.ReadFrames(f.soundBuf)
	if err != nil {
		panic(err)
	}
	return numRead
}
