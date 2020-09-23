package spotifaux

import (
	"github.com/mkb218/gosndfile/sndfile"
)

type wavWriter struct {
	f *sndfile.File
}

func NewWavWriter(filename string) *wavWriter {
	var i sndfile.Info
	i.Format = sndfile.SF_FORMAT_WAV | sndfile.SF_FORMAT_PCM_16
	i.Channels = 1
	i.Samplerate = SAMPLE_RATE
	f, err := sndfile.Open(filename, sndfile.Write, &i)
	if err != nil {
		panic(err)
	}
	return &wavWriter{f: f}
}

func (w *wavWriter) WriteItems(samples []float64) {
	_, err := w.f.WriteItems(samples)
	if err != nil {
		panic(err)
	}
}

func (w *wavWriter) Close() error {
	return w.f.Close()
}
