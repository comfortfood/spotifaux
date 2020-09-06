package main

import (
	"fmt"
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
		fmt.Printf("SoundFile format not supported: %0X\n", f.info.Format)
		return -1
	}
	fmt.Printf("sfinfo.format = %0X, %d\n", f.info.Format, f.info.Channels)
	f.numFrames = f.info.Frames
	if f.numFrames > SF_MAX_NUM_FRAMES {
		f.numFrames = SF_MAX_NUM_FRAMES
	}
	result := f.loadSound()
	return result
}

// Read the entire soundfile into a new float buffer
func (f *soundFile) loadSound() int64 {
	soundBuf := make([]float32, f.numFrames*int64(f.info.Channels))
	numRead, err := f.inFile.ReadFrames(soundBuf)
	if err != nil {
		panic(err)
	}
	return numRead
}

func (f *soundFile) getSoundBuf() ss_sample {
	return f.soundBuf
}

func (f *soundFile) getNumChannels() int {
	return int(f.info.Channels)
}

func (f *soundFile) getBufLen() int64 {
	return f.numFrames // this should be ulong
}
