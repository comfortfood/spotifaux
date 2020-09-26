package spotifaux

import (
	"github.com/runningwild/go-fftw/fftw"
	"math"
)

const CQ_ENV_THRESH = 0.001

type featureExtractor struct {
	bpoN       int       // constant-Q bands per octave (user)
	CqtN       int       // number of constant-Q coefficients (automatic)
	CQT        []float64 // constant-Q transform coefficients
	cqStart    []int     // sparse constant-Q matrix coding indices
	cqStop     []int     // sparse constant-Q matrix coding indices
	DCT        []float64 // discrete cosine transform coefficients
	hammingWin []float64
	winNorm    float64   // Hamming window normalization factor
	SNorm      []float64 // query L2 norm vector
}

func NewFeatureExtractor(sampleRate, fftN, fftOutN int) *featureExtractor {
	e := &featureExtractor{
		bpoN: 12,
	}

	// Construct transform coefficients
	e.makeHammingWin()
	e.makeLogFreqMap(sampleRate, fftN, fftOutN)
	e.makeDCT()

	return e
}

// FFT Hamming window
func (e *featureExtractor) makeHammingWin() {
	e.hammingWin = make([]float64, WindowLength)
	sum := 0.0
	for i := 0; i < WindowLength; i++ {
		e.hammingWin[i] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(WindowLength-1))
		sum += e.hammingWin[i] * e.hammingWin[i]
	}

	e.winNorm = 1.0 / (math.Sqrt(sum * float64(WindowLength)))
}

func (e *featureExtractor) makeLogFreqMap(sampleRate, fftN, fftOutN int) {
	loEdge := 125.0                              // changed from 55.0 * math.Pow(2.0, 2.5/12.0) // low C minus quarter tone
	hiEdge := 7500.0                             // changed from 8000.0
	fratio := math.Pow(2.0, 1.0/float64(e.bpoN)) // Constant-Q bandwidth
	e.CqtN = int(math.Floor(math.Log(hiEdge/loEdge) / math.Log(fratio)))
	fftfrqs := make([]float64, fftOutN)     // Actual number of real FFT coefficients
	logfrqs := make([]float64, e.CqtN)      // Number of constant-Q spectral bins
	logfbws := make([]float64, e.CqtN)      // Bandwidths of constant-Q bins
	e.CQT = make([]float64, e.CqtN*fftOutN) // The transformation matrix
	e.cqStart = make([]int, e.CqtN)         // Sparse matrix coding indices
	e.cqStop = make([]int, e.CqtN)          // Sparse matrix coding indices
	mxnorm := make([]float64, e.CqtN)       // CQ matrix normalization coefficients
	N := float64(fftN)
	for i := 0; i < fftOutN; i++ {
		fftfrqs[i] = float64(i*sampleRate) / N
	}
	for i := 0; i < e.CqtN; i++ {
		logfrqs[i] = loEdge * math.Pow(2.0, float64(i)/float64(e.bpoN))
		logfbws[i] = math.Max(logfrqs[i]*(fratio-1.0), float64(sampleRate)/N)
	}
	ovfctr := 0.5475 // Norm constant so CQT'*CQT close to 1.0
	ptr := 0
	cqEnvThresh := CQ_ENV_THRESH //0.001; // Sparse matrix threshold (for efficient matrix multiplication)

	// Build the constant-Q transform (CQT)
	for i := 0; i < e.CqtN; i++ {
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
	for i := 0; i < e.CqtN; i++ {
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

	nm := 1 / math.Sqrt(float64(e.CqtN)/2.0)

	// Full spectrum DCT matrix
	e.DCT = make([]float64, e.CqtN*e.CqtN)

	for i := 0; i < e.CqtN; i++ {
		for j := 0; j < e.CqtN; j++ {
			e.DCT[i*e.CqtN+j] = nm * math.Cos(float64(i*(2*j+1))*math.Pi/float64(2)/float64(e.CqtN))
		}
	}
	for j := 0; j < e.CqtN; j++ {
		e.DCT[j] *= math.Sqrt(2.0) / 2.0
	}
}

func (e *featureExtractor) computeMFCC(outs1 []float64, fftwPlan *fftw.Plan, fftOutN int, fftComplex *fftw.Array) {

	fftwPlan.Execute()

	// Compute linear power spectrum
	fftPowerSpectrum := make([]float64, fftOutN) // storage for FFT power spectrum
	for i := 0; i < fftOutN; i++ {
		x := real(fftComplex.At(i))     // Real
		y := imag(fftComplex.At(i))     // Imaginary
		fftPowerSpectrum[i] = x*x + y*y // Power
	}

	cqtOut := make([]float64, e.CqtN)
	// sparse matrix product of CQT * FFT
	for i := 0; i < e.CqtN; i++ {
		cqtOut[i] = 0.0
		for j := 0; j < (e.cqStop[i] - e.cqStart[i]); j++ {
			cqtOut[i] += e.CQT[i*fftOutN+e.cqStart[i]+j] * fftPowerSpectrum[e.cqStart[i]+j]
		}
	}

	// LFCC ( in-place )
	for i := 0; i < e.CqtN; i++ {
		cqtOut[i] = math.Log10(cqtOut[i])
	}

	for i := 0; i < e.CqtN; i++ {
		outs1[i] = 0.0
		for j := 0; j < e.CqtN; j++ {
			outs1[i] += cqtOut[j] * e.DCT[i*e.CqtN+j]
		}
	}
}

// extract feature vectors from multichannel audio float buffer (allocate new vector memory)
func (e *featureExtractor) ExtractSeriesOfVectors(s *soundSpotter, fftIn *fftw.Array, fftN int, fftwPlan *fftw.Plan,
	fftOutN int, fftComplex *fftw.Array) {

	e.SNorm = make([]float64, s.LengthSourceShingles)

	for i := 0; i < s.LengthSourceShingles; i++ {
		outputFeatures := s.dbShingles[i]
		power := &s.dbPowers[i]
		buf := make([]float64, WindowLength)
		for j := 0; j < WindowLength; j++ {
			val := 0.0
			if i*Hop+j < len(s.dbBuf) {
				val = s.dbBuf[i*Hop+j] // extract from left channel only
			}
			buf[j] = val
		}

		e.ExtractVector(buf, outputFeatures, power, fftIn, fftN, fftwPlan, fftOutN, fftComplex, s.ChosenFeatures,
			&e.SNorm[i])
	}
	SeriesSqrt(e.SNorm, s.ShingleSize)
}

// extract feature vectors from MONO input buffer
func (e *featureExtractor) ExtractVector(buf, outputFeatures []float64, power *float64, fftIn *fftw.Array,
	fftN int, fftwPlan *fftw.Plan, fftOutN int, fftComplex *fftw.Array, chosenFeatures []int, norm *float64) {

	sum := 0.0
	j := 0
	for ; j < WindowLength; j++ {
		val := buf[j]
		fftIn.Set(j, complex(val*e.hammingWin[j]*e.winNorm, 0))
		sum += val * val
	}
	// zero pad the rest of the FFT window
	for ; j < fftN; j++ {
		fftIn.Set(j, 0)
	}
	*power = sum / float64(WindowLength)                         // power calculation in Bels
	e.computeMFCC(outputFeatures, fftwPlan, fftOutN, fftComplex) // extract MFCC and place result in outputFeatures

	// Keep input L2 norms for correct shingle norming at distance computation stage
	if outputFeatures[chosenFeatures[0]] > NEGINF {
		*norm = VectorSumSquares(outputFeatures, chosenFeatures)
	} else {
		*norm = 0.0
	}
}
