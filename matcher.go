package spotifaux

import (
	"math"
)

// Matching algorithm using recursive matched filter algorithm
// This algorithm is based on factoring the multi-dimensional convolution
// between the current input shingle and the database shingles
//
// The sum-of-products re-factoring reduces the number of multiplications
// required by an order of magnitude or more.
//
// Author: Michael A. Casey, April 24th - November 12th 2006
// Substantially Modified: Michael A. Casey, August 24th - 27th 2007
// Factored out dependency on SoundSpotter class, August 8th - 9th 2009
// Added power features for threshold tests
func Match(wavFileName, datFileName string, s *SoundSpotter) ([]Winner, error) {

	x := len(s.InShingles) / s.ShingleSize
	if len(s.InShingles)%s.ShingleSize > 0 {
		x++
	}
	fileWinners := make([]Winner, x)

	qN := make([]float64, x)
	for ins := 0; ins < x; ins++ {
		for muxi := 0; muxi < s.ShingleSize; muxi++ {
			for _, qp := range s.ChosenFeatures {
				feature := s.InShingles[ins*s.ShingleSize+muxi][qp]
				qN[ins] += feature * feature
			}
		}
		qN[ins] = math.Sqrt(qN[ins])
	}

	dr, err := NewDatReader(datFileName, s.CqtN)
	if err != nil {
		return nil, err
	}

	front := 0
	dbShingles := make([][]float64, s.ShingleSize)
	for dpp := 0; dpp < s.ShingleSize; dpp++ {
		dbShingles[dpp], err = dr.Dat()
		if err != nil {
			return nil, err
		}
	}

	// Make Correlation matrix entry for this frame against entire source database
	for dpp := 0; dpp < dr.Frames; dpp++ {
		for ins := 0; ins < x; ins++ {

			sk := 0.0 // TODO: more efficient with a SeriesSum
			DD := 0.0
			for muxi := 0; muxi < s.ShingleSize; muxi++ {
				m := (front + muxi) % s.ShingleSize
				if dbShingles[m] == nil {
					break
				}
				for _, qp := range s.ChosenFeatures {
					sk += dbShingles[m][qp] * dbShingles[m][qp]
					DD += s.InShingles[ins*s.ShingleSize+muxi][qp] * dbShingles[m][qp]
				}
			}
			sk = math.Sqrt(sk)

			// The norm matched filter distance is the Euclidean distance between the vectors squared Euclidean distance
			dRadius := math.Abs(2 - 2*DD/(qN[ins]*sk))

			// Perform min-dist search
			if (fileWinners[ins] == Winner{}) || dRadius < fileWinners[ins].MinDist {
				fileWinners[ins] = Winner{
					File:    wavFileName,
					MinDist: dRadius,
					Winner:  dpp,
				}
			}
		}

		if dpp+s.ShingleSize < dr.Frames {
			dbShingles[front], err = dr.Dat()
			if err != nil {
				return nil, err
			}
		} else {
			dbShingles[front] = nil
		}
		front = (front + 1) % s.ShingleSize
	}
	return fileWinners, nil
}
