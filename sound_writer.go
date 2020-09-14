package main

import "github.com/mkb218/gosndfile/sndfile"

type wavWriter struct {
	f *sndfile.File
}

func NewWavWriter(name string) *wavWriter {
	var i sndfile.Info
	i.Format = sndfile.SF_FORMAT_WAV | sndfile.SF_FORMAT_PCM_16
	i.Channels = 1
	i.Samplerate = 44100
	f, err := sndfile.Open(name, sndfile.Write, &i)
	if err != nil {
		panic(err)
	}
	return &wavWriter{f: f}
}

func (w *wavWriter) WriteItems(samples ss_sample) {
	_, err := w.f.WriteItems(samples)
	if err != nil {
		panic(err)
	}
}

func (w *wavWriter) Close() error {
	return w.f.Close()
}
