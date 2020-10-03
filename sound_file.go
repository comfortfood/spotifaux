package spotifaux

import (
	"errors"
	"github.com/mkb218/gosndfile/sndfile"
)

type SoundFile struct {
	file   *sndfile.File
	Frames int64
}

// Attempt to open sound file
// return -1 if cannot open SoundFile
// else return number of frames read
func NewSoundFile(fileName string) (*SoundFile, error) {
	sf := &SoundFile{}

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
	if info.Channels > 1 {
		panic("not mono input")
	}
	sf.Frames = info.Frames

	return sf, nil
}

func (f *SoundFile) ReadFrames(out interface{}) (read int64, err error) {
	return f.file.ReadFrames(out)
}

//goland:noinspection GoStandardMethods
func (f *SoundFile) Seek(frames int64) (offset int64, err error) {
	return f.file.Seek(frames, sndfile.Set)
}

func (f *SoundFile) Close() error {
	return f.file.Close()
}
