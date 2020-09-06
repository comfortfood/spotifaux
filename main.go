package main

import (
	"fmt"
	"os"
)

const N = 2048

// test0001: open sound file, set SoundSpotter buffer to SoundFile buffer
func test0001(ss *soundSpotter, sf *soundFile, strbuf string) int {
	retval := sf.sfOpen(strbuf)
	if retval < 0 {
		fmt.Printf("Could not open %s\n", strbuf)
		return 1
	}
	ss.setAudioDatabaseBuf(sf.getSoundBuf(), sf.getBufLen(), sf.getNumChannels())
	return 0
}

// test0002: perform feature extraction on a test sound
func test0002(ss *soundSpotter, strbuf string) int {
	ss.setStatus(EXTRACT)
	fmt.Printf("EXTRACTING %s...", strbuf)

	ss.run(N, nil, nil, nil, nil)
	ss.setStatus(SPOT)
	fmt.Printf("\nDONE.\n")
	return 0
}

func main() {
	var strbuf string
	if len(os.Args) > 1 {
		strbuf = os.Args[1]
	} else {
		strbuf = "/Users/wyatttall/git/BLAST/soundspotter/lib_linux_x86/bell.wav"
	}
	r := newRandomSource()

	ss := newSoundSpotter(44100, N, 2)
	sf := &soundFile{}

	ret := test0001(ss, sf, strbuf)

	// feature extraction
	ret += test0002(ss, strbuf)

	fmt.Printf("%x\n", r.Float64())
	r.Close()
}
