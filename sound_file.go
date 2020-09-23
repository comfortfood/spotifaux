package spotifaux

import (
	"errors"
	"github.com/mkb218/gosndfile/sndfile"
)

const SF_MAX_NUM_FRAMES = SS_MAX_DATABASE_SECS * SAMPLE_RATE

type soundFile struct {
	Channels int
	file     *sndfile.File
	Frames   int64
}

// Attempt to open sound file
// return -1 if cannot open soundFile
// else return number of frames read
func NewSoundFile(fileName string) (*soundFile, error) {
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
	sf.Channels = int(info.Channels)
	sf.Frames = info.Frames
	if sf.Frames > SF_MAX_NUM_FRAMES {
		sf.Frames = SF_MAX_NUM_FRAMES
	}
	return sf, nil
}

func (f *soundFile) ReadFrames(out interface{}) (read int64, err error) {
	return f.file.ReadFrames(out)
}

func (f *soundFile) Close() error {
	return f.file.Close()
}
