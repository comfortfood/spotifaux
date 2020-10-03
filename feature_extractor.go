package spotifaux

import (
	"encoding/binary"
	"github.com/runningwild/go-fftw/fftw"
	"math"
	"os"
)

const CQ_ENV_THRESH = 0.001

type FeatureExtractor struct {
	bpoN       int       // constant-Q bands per octave (user)
	CqtN       int       // number of constant-Q coefficients (automatic)
	CQT        []float64 // constant-Q transform coefficients
	cqStart    []int     // sparse constant-Q matrix coding indices
	cqStop     []int     // sparse constant-Q matrix coding indices
	DCT        []float64 // discrete cosine transform coefficients
	hammingWin []float64
	winNorm    float64 // Hamming window normalization factor
	fftN       int
	fftOutN    int
	fftIn      *fftw.Array
	fftComplex *fftw.Array
	fftwPlan   *fftw.Plan
}

func NewFeatureExtractor(sampleRate int) *FeatureExtractor {

	fftN := SS_FFT_LENGTH // linear frequency resolution (FFT) (user)
	fftOutN := fftN/2 + 1 // linear frequency power spectrum values (automatic)
	fftIn := fftw.NewArray(fftN)
	fftComplex := fftw.NewArray(fftOutN)

	e := &FeatureExtractor{
		bpoN:       12,
		fftN:       fftN,
		fftOutN:    fftOutN,
		fftIn:      fftIn,      // storage for FFT input
		fftComplex: fftComplex, // storage for FFT output
		fftwPlan:   fftw.NewPlan(fftIn, fftComplex, fftw.Forward, fftw.Estimate),
	}

	// Construct transform coefficients
	e.makeHammingWin()
	e.makeLogFreqMap(sampleRate, fftN, fftOutN)
	e.makeDCT()

	return e
}

// FFT Hamming window
func (e *FeatureExtractor) makeHammingWin() {
	e.hammingWin = make([]float64, WindowLength)
	sum := 0.0
	for i := 0; i < WindowLength; i++ {
		e.hammingWin[i] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(WindowLength-1))
		sum += e.hammingWin[i] * e.hammingWin[i]
	}

	e.winNorm = 1.0 / (math.Sqrt(sum * float64(WindowLength)))
}

func (e *FeatureExtractor) makeLogFreqMap(sampleRate, fftN, fftOutN int) {
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
func (e *FeatureExtractor) makeDCT() {

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

// extract feature vectors from multichannel audio float buffer (allocate new vector memory)
func (e *FeatureExtractor) ExtractSeriesOfVectors(wavFileName, datFileName string) error {

	sf, err := NewSoundFile(wavFileName)
	if err != nil {
		return err
	}
	defer sf.Close()

	frames := int(math.Ceil(float64(sf.Frames) / (float64(Hop))))

	dbBuf := make([]float64, sf.Frames)
	_, err = sf.ReadFrames(dbBuf) // TODO: support calling readFrames multiple times
	if err != nil {
		return err
	}

	features := make([][]float64, frames)
	for i := 0; i < frames; i++ {
		features[i] = make([]float64, e.CqtN)
		buf := make([]float64, WindowLength)
		for j := 0; j < WindowLength; j++ {
			val := 0.0
			if i*Hop+j < len(dbBuf) {
				val = dbBuf[i*Hop+j] // extract from left channel only
			}
			buf[j] = val
		}

		e.extractVector(buf, features[i])
	}

	return writeFeatures(datFileName, frames, e.CqtN, features)
}

// extract feature vectors from MONO input buffer
func (e *FeatureExtractor) extractVector(buf, outputFeatures []float64) {

	j := 0
	for ; j < WindowLength; j++ {
		val := buf[j]
		e.fftIn.Set(j, complex(val*e.hammingWin[j]*e.winNorm, 0))
	}
	// zero pad the rest of the FFT window
	for ; j < e.fftN; j++ {
		e.fftIn.Set(j, 0)
	}
	e.computeMFCC(outputFeatures) // extract MFCC and place result in outputFeatures
}

func (e *FeatureExtractor) computeMFCC(outs1 []float64) {

	e.fftwPlan.Execute()

	// Compute linear power spectrum
	fftPowerSpectrum := make([]float64, e.fftOutN) // storage for FFT power spectrum
	for i := 0; i < e.fftOutN; i++ {
		x := real(e.fftComplex.At(i))   // Real
		y := imag(e.fftComplex.At(i))   // Imaginary
		fftPowerSpectrum[i] = x*x + y*y // Power
	}

	cqtOut := make([]float64, e.CqtN)
	// sparse matrix product of CQT * FFT
	for i := 0; i < e.CqtN; i++ {
		cqtOut[i] = 0.0
		for j := 0; j < (e.cqStop[i] - e.cqStart[i]); j++ {
			cqtOut[i] += e.CQT[i*e.fftOutN+e.cqStart[i]+j] * fftPowerSpectrum[e.cqStart[i]+j]
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

func writeFeatures(datFileName string, frames, numFeatures int, features [][]float64) error {

	f, err := os.Create(datFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	fb := make([]byte, 8)
	binary.LittleEndian.PutUint64(fb, uint64(frames))
	_, err = f.Write(fb)
	if err != nil {
		return err
	}

	for i := 0; i < frames; i++ {
		b := make([]byte, 8*numFeatures)
		for j, feature := range features[i] {
			binary.LittleEndian.PutUint64(b[8*j:8+8*j], math.Float64bits(feature))
		}
		_, err = f.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}
