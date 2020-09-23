package spotifaux_test

import (
	"github.com/stretchr/testify/assert"
	"spotifaux"
	"testing"
)

func Test_seriesSum(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5}
	spotifaux.SeriesSum(v, 4)
	assert.Equal(t, v, []float64{10, 14, 12, 9, 5})
}
