package main

import "C"

type idxT C.ulonglong

type seriesOfVectors struct {
	series ss_sample // sample type
	M      idxT      // Length of feature vectors
	N      idxT      // Length of vector series
}

// Column 0 Accessor
func (v *seriesOfVectors) getSeries() ss_sample { return v.series }

// Dimension accessor
func (v *seriesOfVectors) getRows() idxT {
	return v.M
}
