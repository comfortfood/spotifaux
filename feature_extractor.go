package main

import (
	"github.com/runningwild/go-fftw/fftw"
	"math"
)

const CQ_ENV_THRESH = 0.001

type featureExtractor struct {
	fftwPlan         *fftw.Plan
	fftIn            *fftw.Array // storage for FFT input
	fftComplex       *fftw.Array // storage for FFT output
	fftPowerSpectrum []float64   // storage for FFT power spectrum
	fftN             int         // linear frequency resolution (FFT) (user)
	fftOutN          int         // linear frequency power spectrum values (automatic)
	bpoN             int         // constant-Q bands per octave (user)
	cqtN             int         // number of constant-Q coefficients (automatic)
	CQT              []float64   // constant-Q transform coefficients
	cqStart          []int       // sparse constant-Q matrix coding indices
	cqStop           []int       // sparse constant-Q matrix coding indices
	DCT              []float64   // discrete cosine transform coefficients
	cqtOut           []float64   // constant-Q coefficient output storage
	dctOut           ss_sample   // mfcc coefficients (feature output) storage
	logFreqMap       []float64
	loEdge           float64
	hiEdge           float64
	hammingWin       ss_sample
	winNorm          float64 // Hamming window normalization factor
	sampleRate       int
	WindowLength     int
}

func (e *featureExtractor) initializeFeatureExtractor() {
	e.fftOutN = e.fftN/2 + 1
	e.bpoN = 12
	e.cqtN = 0

	// Construct transform coefficients
	e.makeHammingWin()
	e.makeLogFreqMap()
	e.makeDCT()

	// FFTW memory allocation
	e.fftIn = fftw.NewArray(e.fftN)
	e.fftComplex = fftw.NewArray(e.fftOutN)
	e.fftPowerSpectrum = make([]float64, e.fftOutN)
	e.cqtOut = make([]float64, e.cqtN)
	e.dctOut = make(ss_sample, e.cqtN)

	// FFTW plan caching
	e.initializeFFTWplan() // cannot write from VST plugins ?
}

func (e *featureExtractor) initializeFFTWplan() {
	e.fftwPlan = fftw.NewPlan(e.fftIn, e.fftComplex, fftw.Forward, fftw.Estimate)
}

// FFT Hamming window
func (e *featureExtractor) makeHammingWin() {
	TWO_PI := 2 * math.Pi
	oneOverWinLenm1 := 1.0 / float64(e.WindowLength-1)
	if e.hammingWin != nil {
		e.hammingWin = nil
	}
	e.hammingWin = make(ss_sample, e.WindowLength)
	for k := 0; k < e.WindowLength; k++ {
		e.hammingWin[k] = 0.54 - 0.46*math.Cos(TWO_PI*float64(k)*oneOverWinLenm1)
	}
	sum := 0.0
	n := e.WindowLength
	w := 0
	for ; n > 0; n-- {
		sum += e.hammingWin[w] * e.hammingWin[w] // Make a global value, compute only once
		w++
	}
	e.winNorm = 1.0 / (math.Sqrt(sum * float64(e.WindowLength)))
}

func (e *featureExtractor) makeLogFreqMap() {
	var i, j int
	if e.loEdge == 0.0 {
		e.loEdge = 55.0 * math.Pow(2.0, 2.5/12.0) // low C minus quarter tone
	}
	if e.hiEdge == 0.0 {
		e.hiEdge = 8000.0
	}
	fratio := math.Pow(2.0, 1.0/float64(e.bpoN)) // Constant-Q bandwidth
	e.cqtN = int(math.Floor(math.Log(e.hiEdge/e.loEdge) / math.Log(fratio)))
	fftfrqs := make([]float64, e.fftOutN)     // Actual number of real FFT coefficients
	logfrqs := make([]float64, e.cqtN)        // Number of constant-Q spectral bins
	logfbws := make([]float64, e.cqtN)        // Bandwidths of constant-Q bins
	e.CQT = make([]float64, e.cqtN*e.fftOutN) // The transformation matrix
	e.cqStart = make([]int, e.cqtN)           // Sparse matrix coding indices
	e.cqStop = make([]int, e.cqtN)            // Sparse matrix coding indices
	mxnorm := make([]float64, e.cqtN)         // CQ matrix normalization coefficients
	N := float64(e.fftN)
	for i = 0; i < e.fftOutN; i++ {
		fftfrqs[i] = float64(i*e.sampleRate) / N
	}
	for i = 0; i < e.cqtN; i++ {
		logfrqs[i] = e.loEdge * math.Pow(2.0, float64(i)/float64(e.bpoN))
		logfbws[i] = math.Max(logfrqs[i]*(fratio-1.0), float64(e.sampleRate)/N)
	}
	ovfctr := 0.5475 // Norm constant so CQT'*CQT close to 1.0
	var tmp, tmp2 float64
	ptr := 0
	cqEnvThresh := CQ_ENV_THRESH //0.001; // Sparse matrix threshold (for efficient matrix multiplication)
	// Build the constant-Q transform (CQT)
	ptr = 0
	for i = 0; i < e.cqtN; i++ {
		mxnorm[i] = 0.0
		tmp2 = 1.0 / (ovfctr * logfbws[i])
		for j = 0; j < e.fftOutN; j++ {
			tmp = (logfrqs[i] - fftfrqs[j]) * tmp2
			tmp = math.Exp(-0.5 * tmp * tmp)
			e.CQT[ptr] = tmp // row major transform
			mxnorm[i] += tmp * tmp
			ptr++
		}
		mxnorm[i] = 2.0 * math.Sqrt(mxnorm[i])
	}

	// Normalize transform matrix for identity inverse
	ptr = 0
	for i = 0; i < e.cqtN; i++ {
		e.cqStart[i] = 0
		e.cqStop[i] = 0
		tmp = 1.0 / mxnorm[i]
		for j = 0; j < e.fftOutN; j++ {
			e.CQT[ptr] *= tmp
			if e.cqStart[i] == 0 && cqEnvThresh < e.CQT[ptr] {
				e.cqStart[i] = j
			} else if e.cqStop[i] == 0 && e.cqStart[i] != 0 && (e.CQT[ptr] < cqEnvThresh) {
				e.cqStop[i] = j
			}
			ptr++
		}
	}
}

// discrete cosine transform
func (e *featureExtractor) makeDCT() {
	var i, j int
	nm := 1 / math.Sqrt(float64(e.cqtN)/2.0)
	// Full spectrum DCT matrix
	e.DCT = make([]float64, e.cqtN*e.cqtN)

	for i = 0; i < e.cqtN; i++ {
		for j = 0; j < e.cqtN; j++ {
			e.DCT[i*e.cqtN+j] = nm * math.Cos(float64(i*(2*j+1))*math.Pi/float64(2)/float64(e.cqtN))
		}
	}
	for j = 0; j < e.cqtN; j++ {
		e.DCT[j] *= math.Sqrt(2.0) / 2.0
	}
}

func (e *featureExtractor) computeMFCC(outs1 ss_sample) {
	var x, y float64

	e.fftwPlan.Execute()

	cp := 0 // the FFTW output
	op := 0 // the MFCC output
	// Compute linear power spectrum
	c := e.fftOutN
	for ; c > 0; c-- {
		x = real(e.fftComplex.At(cp))      // Real
		y = imag(e.fftComplex.At(cp))      // Imaginary
		e.fftPowerSpectrum[op] = x*x + y*y // Power
		op++
		cp++
	}

	var a, b int
	var ptr1, ptr2, ptr3 int
	mfccPtr := 0

	// sparse matrix product of CQT * FFT
	for a = 0; a < e.cqtN; a++ {
		ptr1 = a // constant-Q transform vector
		e.cqtOut[a] = 0.0
		ptr2 = a*e.fftOutN + e.cqStart[a]
		ptr3 = e.cqStart[a]
		b = e.cqStop[a] - e.cqStart[a]
		for ; b > 0; b-- {
			e.cqtOut[ptr1] += e.CQT[ptr2] * e.fftPowerSpectrum[ptr3]
			ptr2++
			ptr3++
		}
	}

	// LFCC ( in-place )
	a = e.cqtN
	ptr1 = 0
	for ; a > 0; a-- {
		e.cqtOut[ptr1] = math.Log10(e.cqtOut[ptr1])
		ptr1++
	}
	a = e.cqtN
	ptr2 = 0 // point to column of DCT
	mfccPtr = 0
	for ; a > 0; a-- {
		ptr1 = 0 // point to cqt vector
		outs1[mfccPtr] = 0.0
		b = e.cqtN
		for ; b > 0; b-- {
			outs1[mfccPtr] += e.cqtOut[ptr1] * e.DCT[ptr2]
			ptr1++
			ptr2++
		}
		mfccPtr++
	}
}

// extract feature vectors from multichannel audio float buffer (allocate new vector memory)
func (e *featureExtractor) extractSeriesOfVectors(s *soundSpotter) {
	var ptr1, ptr2 int                                   // moving pointer to hamming window
	oneOverWindowLength := 1.0 / float64(e.WindowLength) // power normalization
	var xPtr, dbSize int
	for int64(xPtr) <= s.bufLen-int64(e.WindowLength)*int64(s.numChannels) && dbSize <= s.getLengthSourceShingles() {
		o := 0
		in := xPtr
		w := 0
		n2 := e.WindowLength
		var val, sum float64
		for ; n2 > 0; n2-- {
			val = s.audioDatabaseBuf[in]
			e.fftIn.Set(o, complex(val*e.hammingWin[w]*e.winNorm, 0))
			o++
			w++
			sum += val * val
			in += s.numChannels // extract from left channel only
		}
		s.dbPowers[dbSize] = sum * float64(oneOverWindowLength) // database powers calculation in Bels
		n2 = e.fftN - e.WindowLength                            // Zero pad the rest of the FFT window
		for ; n2 > 0; n2-- {
			e.fftIn.Set(o, 0)
			o++
		}
		e.computeMFCC(e.dctOut)
		ptr1 = dbSize * e.cqtN
		ptr2 = 0
		n2 = e.cqtN
		for ; n2 > 0; n2-- { // Copy to series of vectors
			s.dbShingles.series[ptr1] = e.dctOut[ptr2]
			ptr1++
			ptr2++
		}
		xPtr += e.WindowLength * s.numChannels
		dbSize++
	}
	s.dbSize = dbSize
	s.normsNeedUpdate = true
}

// extract feature vectors from MONO input buffer
func (e *featureExtractor) extractVector(n int, inputSamps, outputFeatures ss_sample, power *float64) {
	w := 0
	o := 0
	in := 0 // the MONO input buffer
	var val, sum float64
	oneOverWindowLength := 1.0 / float64(e.WindowLength)
	// window input samples
	for ; n > 0; n-- {
		val = inputSamps[in]
		in++
		sum += val * val
		c := complex(val*e.hammingWin[w]*e.winNorm, 0)
		e.fftIn.Set(o, c)
		o++
		w++
	}
	*power = sum * oneOverWindowLength // power calculation in Bels
	// zero pad the rest of the FFT window
	n = e.fftN - e.WindowLength
	for ; n > 0; n-- {
		e.fftIn.Set(o, 0)
	} // <====== FIX ME: This is redundant (in theory, but pointers might get re-used in SP chain)
	// extract MFCC and place result in outputFeatures
	e.computeMFCC(outputFeatures)
}
