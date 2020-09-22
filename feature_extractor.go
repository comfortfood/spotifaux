package main

import (
	"github.com/runningwild/go-fftw/fftw"
	"math"
)

const CQ_ENV_THRESH = 0.001

type featureExtractor struct {
	bpoN         int       // constant-Q bands per octave (user)
	cqtN         int       // number of constant-Q coefficients (automatic)
	CQT          []float64 // constant-Q transform coefficients
	cqStart      []int     // sparse constant-Q matrix coding indices
	cqStop       []int     // sparse constant-Q matrix coding indices
	DCT          []float64 // discrete cosine transform coefficients
	cqtOut       []float64 // constant-Q coefficient output storage
	logFreqMap   []float64
	loEdge       float64
	hiEdge       float64
	hammingWin   []float64
	winNorm      float64 // Hamming window normalization factor
	sampleRate   int
	WindowLength int
}

func newFeatureExtractor(sampleRate, WindowLength, fftN, fftOutN int) *featureExtractor {
	e := &featureExtractor{
		sampleRate:   sampleRate,
		WindowLength: WindowLength,
		bpoN:         12,
	}

	// Construct transform coefficients
	e.makeHammingWin()
	e.makeLogFreqMap(fftN, fftOutN)
	e.makeDCT()

	e.cqtOut = make([]float64, e.cqtN)

	return e
}

// FFT Hamming window
func (e *featureExtractor) makeHammingWin() {
	e.hammingWin = make([]float64, e.WindowLength)
	sum := 0.0
	for i := 0; i < e.WindowLength; i++ {
		e.hammingWin[i] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(e.WindowLength-1))
		sum += e.hammingWin[i] * e.hammingWin[i]
	}

	e.winNorm = 1.0 / (math.Sqrt(sum * float64(e.WindowLength)))
}

func (e *featureExtractor) makeLogFreqMap(fftN, fftOutN int) {
	e.loEdge = 55.0 * math.Pow(2.0, 2.5/12.0) // low C minus quarter tone
	e.hiEdge = 8000.0
	fratio := math.Pow(2.0, 1.0/float64(e.bpoN)) // Constant-Q bandwidth
	e.cqtN = int(math.Floor(math.Log(e.hiEdge/e.loEdge) / math.Log(fratio)))
	fftfrqs := make([]float64, fftOutN)     // Actual number of real FFT coefficients
	logfrqs := make([]float64, e.cqtN)      // Number of constant-Q spectral bins
	logfbws := make([]float64, e.cqtN)      // Bandwidths of constant-Q bins
	e.CQT = make([]float64, e.cqtN*fftOutN) // The transformation matrix
	e.cqStart = make([]int, e.cqtN)         // Sparse matrix coding indices
	e.cqStop = make([]int, e.cqtN)          // Sparse matrix coding indices
	mxnorm := make([]float64, e.cqtN)       // CQ matrix normalization coefficients
	N := float64(fftN)
	for i := 0; i < fftOutN; i++ {
		fftfrqs[i] = float64(i*e.sampleRate) / N
	}
	for i := 0; i < e.cqtN; i++ {
		logfrqs[i] = e.loEdge * math.Pow(2.0, float64(i)/float64(e.bpoN))
		logfbws[i] = math.Max(logfrqs[i]*(fratio-1.0), float64(e.sampleRate)/N)
	}
	ovfctr := 0.5475 // Norm constant so CQT'*CQT close to 1.0
	ptr := 0
	cqEnvThresh := CQ_ENV_THRESH //0.001; // Sparse matrix threshold (for efficient matrix multiplication)

	// Build the constant-Q transform (CQT)
	for i := 0; i < e.cqtN; i++ {
		mxnorm[i] = 0.0
		tmp2 := 1.0 / (ovfctr * logfbws[i])
		for j := 0; j < fftOutN; j++ {
			tmp := (logfrqs[i] - fftfrqs[j]) * tmp2
			tmp = math.Exp(-0.5 * tmp * tmp)
			e.CQT[ptr] = tmp // row major transform
			mxnorm[i] += tmp * tmp
			ptr++
		}
		mxnorm[i] = 2.0 * math.Sqrt(mxnorm[i])
	}

	// Normalize transform matrix for identity inverse
	ptr = 0
	for i := 0; i < e.cqtN; i++ {
		e.cqStart[i] = 0
		e.cqStop[i] = 0
		tmp := 1.0 / mxnorm[i]
		for j := 0; j < fftOutN; j++ {
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
	nm := 1 / math.Sqrt(float64(e.cqtN)/2.0)
	// Full spectrum DCT matrix
	e.DCT = make([]float64, e.cqtN*e.cqtN)

	for i := 0; i < e.cqtN; i++ {
		for j := 0; j < e.cqtN; j++ {
			e.DCT[i*e.cqtN+j] = nm * math.Cos(float64(i*(2*j+1))*math.Pi/float64(2)/float64(e.cqtN))
		}
	}
	for j := 0; j < e.cqtN; j++ {
		e.DCT[j] *= math.Sqrt(2.0) / 2.0
	}
}

func (e *featureExtractor) computeMFCC(outs1 []float64, fftwPlan *fftw.Plan, fftOutN int, fftComplex *fftw.Array, fftPowerSpectrum []float64) {

	fftwPlan.Execute()

	// Compute linear power spectrum
	for i := 0; i < fftOutN; i++ {
		x := real(fftComplex.At(i))     // Real
		y := imag(fftComplex.At(i))     // Imaginary
		fftPowerSpectrum[i] = x*x + y*y // Power
	}

	// sparse matrix product of CQT * FFT
	for i := 0; i < e.cqtN; i++ {
		e.cqtOut[i] = 0.0
		for j := 0; j < (e.cqStop[i] - e.cqStart[i]); j++ {
			e.cqtOut[i] += e.CQT[i*fftOutN+e.cqStart[i]+j] * fftPowerSpectrum[e.cqStart[i]+j]
		}
	}

	// LFCC ( in-place )
	for i := 0; i < e.cqtN; i++ {
		e.cqtOut[i] = math.Log10(e.cqtOut[i])
	}

	for i := 0; i < e.cqtN; i++ {
		outs1[i] = 0.0
		for j := 0; j < e.cqtN; j++ {
			outs1[i] += e.cqtOut[j] * e.DCT[i*e.cqtN+j]
		}
	}
}

// extract feature vectors from multichannel audio float buffer (allocate new vector memory)
func (e *featureExtractor) extractSeriesOfVectors(s *soundSpotter, fftIn *fftw.Array, fftN int, fftwPlan *fftw.Plan,
	fftOutN int, fftComplex *fftw.Array, fftPowerSpectrum []float64) {
	i := 0
	for ; i < s.getLengthSourceShingles()-1; i++ {
		sum := 0.0
		j := 0
		for ; j < e.WindowLength; j++ {
			val := s.dbBuf[(i*e.WindowLength+j)*s.channels] // extract from left channel only
			fftIn.Set(j, complex(val*e.hammingWin[j]*e.winNorm, 0))
			sum += val * val
		}
		for ; j < fftN; j++ {
			fftIn.Set(j, 0) // Zero pad the rest of the FFT window
		}
		s.dbPowers[i] = sum / float64(e.WindowLength) // database powers calculation in Bels
		s.dbPowersCurrent[i] = sum / float64(e.WindowLength)
		dbShingle := s.dbShingles[i]
		e.computeMFCC(dbShingle, fftwPlan, fftOutN, fftComplex, fftPowerSpectrum)
	}
	seriesMean(s.dbPowersCurrent, s.shingleSize, s.getLengthSourceShingles())
	s.dbSize = i
}

// extract feature vectors from MONO input buffer
func (e *featureExtractor) extractVector(inputSamps, outputFeatures []float64, power *float64, fftIn *fftw.Array,
	fftN int, fftwPlan *fftw.Plan, fftOutN int, fftComplex *fftw.Array, fftPowerSpectrum []float64) {
	sum := 0.0
	i := 0
	for ; i < e.WindowLength; i++ {
		val := inputSamps[i]
		sum += val * val
		fftIn.Set(i, complex(val*e.hammingWin[i]*e.winNorm, 0))
	}
	// zero pad the rest of the FFT window
	for ; i < fftN; i++ {
		fftIn.Set(i, 0)
	}
	*power = sum / float64(e.WindowLength)                                         // power calculation in Bels
	e.computeMFCC(outputFeatures, fftwPlan, fftOutN, fftComplex, fftPowerSpectrum) // extract MFCC and place result in outputFeatures
}
