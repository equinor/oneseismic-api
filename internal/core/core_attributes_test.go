package core

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSurfaceUnalignedWithSeismic(t *testing.T) {
	const above = float32(4.0)
	const below = float32(4.0)
	const stepsize = float32(4.0)
	var targetAttributes = []string{"samplevalue"}

	expected := []float32{
		fillValue, fillValue, fillValue, fillValue, fillValue, fillValue, fillValue,
		fillValue, fillValue, -12.5, fillValue, 4.5, fillValue, 1.5,
		fillValue, fillValue, 12.5, fillValue, -10.5, fillValue, -1.5,
	}

	values := [][]float32{
		{fillValue, fillValue, fillValue, fillValue, fillValue, fillValue, fillValue},
		{fillValue, fillValue, 16, fillValue, 16, fillValue, 16},
		{fillValue, fillValue, 16, fillValue, 16, fillValue, 16},
	}

	rotation := samples10_grid.rotation + 270
	xinc := samples10_grid.xinc / 2.0
	yinc := -samples10_grid.yinc
	xori := float32(16)
	yori := float32(18)

	surface := RegularSurface{
		Values:    values,
		Rotation:  &rotation,
		Xori:      &xori,
		Yori:      &yori,
		Xinc:      xinc,
		Yinc:      yinc,
		FillValue: &fillValue,
	}

	interpolationMethod, _ := GetInterpolationMethod("nearest")

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.Len(t, buf, len(targetAttributes), "Wrong number of attributes")
	require.NoErrorf(t, err, "Failed to fetch horizon")
	result, err := toFloat32(buf[0])
	require.NoErrorf(t, err, "Failed to covert to float32 buffer")
	require.Equalf(t, expected, *result, "Horizon not as expected")
}

func TestSurfaceWindowVerticalBounds(t *testing.T) {
	testcases := []struct {
		name     string
		values   [][]float32
		below    float32
		above    float32
		inbounds bool
	}{
		// top boundary at 4.0, bottom boundary at 40.0
		{
			name:     "value - above is 2 samples away from the first existing sample",
			values:   [][]float32{{12.00}},
			above:    0,
			below:    12,
			inbounds: true,
		},
		{
			name:     "value - above snaps to sample which is 2 samples away from the first existing sample",
			values:   [][]float32{{9.00}},
			above:    0,
			below:    12,
			inbounds: true,
		},
		{
			name:     "value - above is 1 sample away from the first existing sample",
			values:   [][]float32{{8.00}},
			above:    0,
			below:    12,
			inbounds: true,
		},
		{
			name:     "value - above snaps to sample which is 1 sample away from the first existing sample",
			values:   [][]float32{{7.00}},
			above:    0,
			below:    12,
			inbounds: true,
		},
		{
			name:     "value - above is on the first existing sample",
			values:   [][]float32{{4.00}},
			above:    0,
			below:    12,
			inbounds: true,
		},
		{
			name:     "value - above is less than first existing sample",
			values:   [][]float32{{3.00}},
			above:    0,
			below:    12,
			inbounds: false,
		},
		{
			name:     "value + below is 2 samples away from the last existing sample",
			values:   [][]float32{{32.00}},
			above:    12,
			below:    0,
			inbounds: true,
		},
		{
			name:     "value + below snaps to sample which is 2 samples away from the last existing sample",
			values:   [][]float32{{35.00}},
			above:    12,
			below:    0,
			inbounds: true,
		},
		{
			name:     "value + below is 1 sample away from the last existing sample",
			values:   [][]float32{{36.00}},
			above:    12,
			below:    0,
			inbounds: true,
		},
		{
			name:     "value + below snaps to sample which is 1 sample away from the last existing sample",
			values:   [][]float32{{38.00}},
			above:    12,
			below:    0,
			inbounds: true,
		},
		{
			name:     "value + below is on the last existing sample",
			values:   [][]float32{{40.00}},
			above:    12,
			below:    0,
			inbounds: true,
		},
		{
			name:     "value + below is greater than last existing sample",
			values:   [][]float32{{41.00}},
			above:    12,
			below:    0,
			inbounds: false,
		},
		{
			name:     "Some values inbounds, some out of bounds",
			values:   [][]float32{{22.00, 40.00, 8.00}, {41.00, 35.00, 32.00}, {12.00, 11.00, 7.00}},
			above:    4,
			below:    4,
			inbounds: false,
		},
		{
			name:     "For attributes fillvalue means 'ignore trace' and should not be bounds checked",
			values:   [][]float32{{-999.25}},
			above:    4,
			below:    4,
			inbounds: true,
		},
	}

	targetAttributes := []string{"min", "samplevalue"}
	const stepsize = float32(4.0)

	for _, testcase := range testcases {
		surface := samples10Surface(testcase.values)
		interpolationMethod, _ := GetInterpolationMethod("nearest")
		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		_, boundsErr := handle.GetAttributesAlongSurface(
			surface,
			testcase.above,
			testcase.below,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)

		if testcase.inbounds {
			require.NoErrorf(t, boundsErr,
				"[%s] Expected horizon value %f to be in bounds",
				testcase.name,
				testcase.values[0][0],
			)
		}

		if !testcase.inbounds {
			require.ErrorContainsf(t, boundsErr,
				"out of vertical bound",
				"[%s] Expected horizon value %v to throw out of bound",
				testcase.name,
				testcase.values,
			)
		}
	}
}

/**
 * The goal of this test is to test that the bounds checking in the horizontal
 * plane is correct - i.e. that we correctly populate the output array with
 * fillvalues. To achieve this we create horizons that are based on the VDS's
 * bingrid and then translated them in the XY-domain (by moving around the
 * origin of the horizon).
 *
 * Anything up to half a voxel outside the VDS' range is allowed. More than
 * half a voxel is considered out-of-bounds. In world coordinates, that
 * corresponds to half a bingrid. E.g. in the figure below we move the origin
 * (point 0) along the vector from 0 - 2. Moving 0 half the distance between 0
 * and 2 corresponds to moving half a voxel, and in this case that will leave
 * point 4 and 5 half a voxel out of bounds. This test does this exercise in
 * all direction, testing around 0.5 offset.
 *
 *      ^   VDS bingrid          ^    Translated horizon
 *      |                        |
 *      |                     13 -           5
 *      |                        |          / \
 *   11 -         5              |         /   4
 *      |        / \             |        /   /
 *      |       /   4          9 -       3   /
 *      |      /   /             |      / \ /
 *    7 -     3   /              |     /   2
 *      |    / \ /               |    /   /
 *      |   /   2              5 -   1   /
 *      |  /   /                 |    \ /
 *    3 - 1   /                  |     0
 *      |  \ /                   |
 *      |   0                    |
 *      +-|---------|--->        +-|---------|--->
 *        0         14             0        14
 *
 */
func TestSurfaceHorizontalBounds(t *testing.T) {
	xinc := float64(samples10_grid.xinc)
	yinc := float64(samples10_grid.yinc)
	rot := float64(samples10_grid.rotation)
	rotrad := rot * math.Pi / 180

	values := [][]float32{{16, 16}, {16, 16}, {16, 16}}

	targetAttributes := []string{"samplevalue"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	const above = float32(4.0)
	const below = float32(4.0)
	const stepsize = float32(4.0)

	testcases := []struct {
		name     string
		xori     float64
		yori     float64
		expected []float32
	}{
		{
			name:     "X coordinate is almost half a bingrid too high",
			xori:     2.0 + 0.49*xinc*math.Cos(rotrad),
			yori:     0.0 + 0.49*xinc*math.Sin(rotrad),
			expected: []float32{-1.5, 1.5, -10.5, 4.5, 12.5, -12.5},
		},
		{
			name:     "X coordinate is more than half a bingrid too high",
			xori:     2.0 + 0.51*xinc*math.Cos(rotrad),
			yori:     0.0 + 0.51*xinc*math.Sin(rotrad),
			expected: []float32{-10.5, 4.5, 12.5, -12.5, fillValue, fillValue},
		},
		{
			name:     "X coordinate is almost half a bingrid too low",
			xori:     2.0 - 0.49*xinc*math.Cos(rotrad),
			yori:     0.0 - 0.49*xinc*math.Sin(rotrad),
			expected: []float32{-1.5, 1.5, -10.5, 4.5, 12.5, -12.5},
		},
		{
			name:     "X coordinate is more than half a bingrid too low",
			xori:     2.0 - 0.51*xinc*math.Cos(rotrad),
			yori:     0.0 - 0.51*xinc*math.Sin(rotrad),
			expected: []float32{fillValue, fillValue, -1.5, 1.5, -10.5, 4.5},
		},
		{
			name:     "Y coordinate is almost half a bingrid too high",
			xori:     2.0 + 0.49*yinc*-math.Sin(rotrad),
			yori:     0.0 + 0.49*yinc*math.Cos(rotrad),
			expected: []float32{-1.5, 1.5, -10.5, 4.5, 12.5, -12.5},
		},
		{
			name:     "Y coordinate is more than half a bingrid too high",
			xori:     2.0 + 0.51*yinc*-math.Sin(rotrad),
			yori:     0.0 + 0.51*yinc*math.Cos(rotrad),
			expected: []float32{1.5, fillValue, 4.5, fillValue, -12.5, fillValue},
		},
		{
			name:     "Y coordinate is almost half a bingrid too low",
			xori:     2.0 - 0.49*yinc*-math.Sin(rotrad),
			yori:     0.0 - 0.49*yinc*math.Cos(rotrad),
			expected: []float32{-1.5, 1.5, -10.5, 4.5, 12.5, -12.5},
		},
		{
			name:     "Y coordinate is more than half a bingrid too low",
			xori:     2.0 - 0.51*yinc*-math.Sin(rotrad),
			yori:     0.0 - 0.51*yinc*math.Cos(rotrad),
			expected: []float32{fillValue, -1.5, fillValue, -10.5, fillValue, 12.5},
		},
	}

	for _, testcase := range testcases {
		rot32 := float32(rot)
		xori32 := float32(testcase.xori)
		yori32 := float32(testcase.yori)
		surface := RegularSurface{
			Values:    values,
			Rotation:  &rot32,
			Xori:      &xori32,
			Yori:      &yori32,
			Xinc:      float32(xinc),
			Yinc:      float32(yinc),
			FillValue: &fillValue,
		}
		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		buf, err := handle.GetAttributesAlongSurface(
			surface,
			above,
			below,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)
		require.NoErrorf(t, err,
			"[%s] Failed to fetch horizon, err: %v",
			testcase.name,
			err,
		)

		require.Len(t, buf, len(targetAttributes), "Wrong number of attributes")

		result, err := toFloat32(buf[0])
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.Equalf(
			t,
			testcase.expected,
			*result,
			"[%v]",
			targetAttributes[0],
		)
	}
}

func TestAttribute(t *testing.T) {
	targetAttributes := []string{
		"samplevalue",
		"min",
		"min_at",
		"max",
		"max_at",
		"maxabs",
		"maxabs_at",
		"mean",
		"meanabs",
		"meanpos",
		"meanneg",
		"median",
		"rms",
		"var",
		"sd",
		"sumpos",
		"sumneg",
	}
	expected := [][]float32{
		{-0.5, 0.5, -8.5, 6.5, fillValue, -16.5, fillValue, fillValue},                        // samplevalue
		{-2.5, -1.5, -12.5, 2.5, fillValue, -24.5, fillValue, fillValue},                      // min
		{12, 28, 12, 12, fillValue, 28, fillValue, fillValue},                                 // min_at
		{1.5, 2.5, -4.5, 10.5, fillValue, -8.5, fillValue, fillValue},                         // max
		{28, 12, 28, 28, fillValue, 12, fillValue, fillValue},                                 // max_at
		{2.5, 2.5, 12.5, 10.5, fillValue, 24.5, fillValue, fillValue},                         // maxabs
		{12, 12, 12, 28, fillValue, 28, fillValue, fillValue},                                 // maxabs_at
		{-0.5, 0.5, -8.5, 6.5, fillValue, -16.5, fillValue, fillValue},                        // mean
		{1.3, 1.3, 8.5, 6.5, fillValue, 16.5, fillValue, fillValue},                           // meanabs
		{1, 1.5, 0, 6.5, fillValue, 0, fillValue, fillValue},                                  // meanpos
		{-1.5, -1, -8.5, 0, fillValue, -16.5, fillValue, fillValue},                           // meanneg
		{-0.5, 0.5, -8.5, 6.5, fillValue, -16.5, fillValue, fillValue},                        // median
		{1.5, 1.5, 8.958237, 7.0887237, fillValue, 17.442764, fillValue, fillValue},           // rms
		{2, 2, 8, 8, fillValue, 32, fillValue, fillValue},                                     // var
		{1.4142135, 1.4142135, 2.828427, 2.828427, fillValue, 5.656854, fillValue, fillValue}, // sd
		{2, 4.5, 0, 32.5, fillValue, 0, fillValue, fillValue},                                 // sumpos
		{-4.5, -2, -42.5, 0, fillValue, -82.5, fillValue, fillValue},                          // sumneg
	}

	values := [][]float32{
		{20, 20},
		{20, 20},
		{fillValue, 20},
		{20, 20}, // Out-of-bounds, should return fillValue
	}

	surface := samples10Surface(values)

	interpolationMethod, _ := GetInterpolationMethod("nearest")
	const above = float32(8.0)
	const below = float32(8.0)
	const stepsize = float32(4.0)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err, "Failed to fetch horizon, err %v", err)
	require.Len(t, buf, len(targetAttributes),
		"Incorrect number of attributes returned",
	)

	for i, attr := range buf {
		result, err := toFloat32(attr)
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.InDeltaSlicef(
			t,
			expected[i],
			*result,
			0.000001,
			"[%s]\nExpected: %v\nActual:   %v",
			targetAttributes[i],
			expected[i],
			*result,
		)
	}
}

func TestAttributeMedianForEvenSampleValue(t *testing.T) {
	targetAttributes := []string{"median"}
	expected := [][]float32{
		{-1, 1, -9.5, 5.5, fillValue, -14.5, fillValue, fillValue}, // median
	}

	values := [][]float32{
		{20, 20},
		{20, 20},
		{fillValue, 20},
		{20, 20}, // Out-of-bounds, should return fillValue
	}

	surface := samples10Surface(values)

	interpolationMethod, _ := GetInterpolationMethod("nearest")
	const above = float32(8.0)
	const below = float32(4.0)
	const stepsize = float32(4.0)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err, "Failed to fetch horizon, err %v", err)
	require.Len(t, buf, len(targetAttributes),
		"Incorrect number of attributes returned",
	)

	for i, attr := range buf {
		result, err := toFloat32(attr)
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.InDeltaSlicef(
			t,
			expected[i],
			*result,
			0.000001,
			"[%s]\nExpected: %v\nActual:   %v",
			targetAttributes[i],
			expected[i],
			*result,
		)
	}
}

func TestAttributesAboveBelowStepSizeIgnoredForSampleValue(t *testing.T) {
	testCases := []struct {
		name     string
		above    float32
		below    float32
		stepsize float32
	}{
		{
			name:     "Everything is zero",
			above:    0,
			below:    0,
			stepsize: 0,
		},
		{
			name:     "stepsize is none-zero and horizon is unaligned",
			above:    0,
			below:    0,
			stepsize: 3,
		},
		{
			name:     "above/below is none-zero, stepsize is not and horizon is unaligned",
			above:    8,
			below:    8,
			stepsize: 0,
		},
		{
			name:     "Everything is none-zero and horizon is unaligned",
			above:    7,
			below:    8,
			stepsize: 2,
		},
	}

	values := [][]float32{{26.0}}
	expected := [][]float32{{1.0}}

	targetAttributes := []string{"samplevalue"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	surface := samples10Surface(values)

	for _, testCase := range testCases {
		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		buf, err := handle.GetAttributesAlongSurface(
			surface,
			testCase.above,
			testCase.below,
			testCase.stepsize,
			targetAttributes,
			interpolationMethod,
		)
		require.NoErrorf(t, err,
			"[%s] Failed to fetch horizon, err: %v", testCase.name, err,
		)
		require.Len(t, buf, len(targetAttributes),
			"Incorrect number of attributes returned",
		)

		for i, attr := range buf {
			result, err := toFloat32(attr)
			require.NoErrorf(t, err, "Couldn't convert to float32")

			require.InDeltaSlicef(
				t,
				expected[i],
				*result,
				0.000001,
				"[%s][%s]\nExpected: %v\nActual:   %v",
				testCase.name,
				targetAttributes[i],
				expected[i],
				*result,
			)
		}
	}
}

/** The vertical domain of the surface is perfectly aligned with the seismic
 *  but subsampled
 *
 *  seismic trace   surface window(s)
 *  -------------   ---------------
 *
 *       04ms -
 *            |
 *            |      rate: 2ms   rate: 1ms  rate: ....
 *            |
 *       08ms -      - 08ms      - 08ms
 *            |      |           - 09ms
 *            |      - 10ms      - 10ms
 *            |      |           - 11ms
 *       12ms -      - 12ms      - 12ms
 *            |      |           - 13ms
 *            |      - 14ms      - 14ms
 *            |      |           - 15ms
 *       16ms -      - 16ms      - 16ms
 *            |      |           - 17ms
 *            |      - 18ms      - 18ms
 *            |      |           - 19ms
 *       20ms -      x 20ms      x 20ms
 *            |      |           - 21ms
 *            |      - 22ms      - 22ms
 *            |      |           - 23ms
 *       24ms -      - 24ms      - 24ms
 *            |
 *            |
 *            |
 *       28ms -
 *            |
 *            v
 */
func TestAttributeSubsamplingAligned(t *testing.T) {
	testCases := []struct {
		name     string
		stepsize float32
	}{
		{
			name:     "stepsize: 4.0",
			stepsize: 4.0,
		},
		{
			name:     "stepsize: 2.0",
			stepsize: 2.0,
		},
		{
			name:     "stepsize: 1.0",
			stepsize: 1.0,
		},
		{
			name:     "stepsize: 0.5",
			stepsize: 0.5,
		},
		{
			name:     "stepsize: 0.2",
			stepsize: 0.2,
		},
		{
			name:     "stepsize: 0.1",
			stepsize: 0.1,
		},
	}

	const above = float32(8)
	const below = float32(4)
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	targetAttributes := []string{"min", "max"}

	values := [][]float32{
		{20, 20},
		{20, 20},
		{fillValue, 20},
		{20, 20}, // Out-of-bounds, should return fillValue
	}

	expected := [][]float32{
		{-2.5, -0.5, -12.5, 2.5, fillValue, -20.5, fillValue, fillValue}, // min
		{0.5, 2.5, -6.5, 8.5, fillValue, -8.5, fillValue, fillValue},     // max
	}

	surface := samples10Surface(values)

	for _, testCase := range testCases {
		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		buf, err := handle.GetAttributesAlongSurface(
			surface,
			above,
			below,
			testCase.stepsize,
			targetAttributes,
			interpolationMethod,
		)
		require.NoErrorf(t, err,
			"[%s] Failed to fetch horizon, err: %v", testCase.name, err,
		)
		require.Len(t, buf, len(targetAttributes),
			"Incorrect number of attributes returned",
		)

		for i, attr := range buf {
			result, err := toFloat32(attr)
			require.NoErrorf(t, err, "[%v] Couldn't convert to float32", testCase.name)

			require.InDeltaSlicef(
				t,
				expected[i],
				*result,
				0.000001,
				"[%s][%s]\nExpected: %v\nActual:   %v",
				testCase.name,
				targetAttributes[i],
				expected[i],
				*result,
			)
		}
	}
}

/** The vertical domain of the surface is unaligned with the seismic
 *  and target stepsize is higher than that of the seismic
 *
 *  seismic trace   surface window(s)
 *  -------------   ---------------
 *
 *       04ms -
 *            |
 *            |
 *            |
 *       08ms -
 *            |      - 09ms
 *            |      |
 *            |      - 11ms
 *       12ms -      |
 *            |      - 13ms
 *            |      |
 *            |      - 15ms
 *       16ms -      |
 *            |      - 17ms
 *            |      |
 *            |      - 19ms
 *       20ms -      |
 *            |      x 21ms
 *            |      |
 *            |      - 23ms
 *       24ms -      |
 *            |      - 25ms
 *            |
 *            |
 *       28ms -
 *            |
 *            v
 */
func TestAttributesUnalignedAndSubsampled(t *testing.T) {
	expected := [][]float32{
		{-2.25}, // min
		{0.75},  // max
		{-0.75}, // mean
	}
	const above = float32(8)
	const below = float32(4)
	const stepsize = float32(2)

	targetAttributes := []string{"min", "max", "mean"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	values := [][]float32{{21}}

	surface := samples10Surface(values)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err, "Failed to fetch horizon, err: %v", err)
	require.Len(t, buf, len(targetAttributes),
		"Incorrect number of attributes returned",
	)

	for i, attr := range buf {
		result, err := toFloat32(attr)
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.InDeltaSlicef(
			t,
			expected[i],
			*result,
			0.000001,
			"[%s]\nExpected: %v\nActual:   %v",
			targetAttributes[i],
			expected[i],
			*result,
		)
	}
}

/** The vertical domain of the surface is unaligned with the seismic
 *  but share the stepsize
 *
 *  seismic trace   surface window
 *  -------------   --------------
 *
 *            ^
 *            |
 *       08ms -
 *            |       - 9ms
 *            |       |           - 10ms
 *            |       |           |           - 11ms
 *       12ms -       |           |           |
 *            |       - 13ms      |           |
 *            |       |           - 14ms      |
 *            |       |           |           - 15ms
 *       16ms -       |           |           |
 *            |       - 17ms      |           |
 *            |       |           - 18ms      |
 *            |       |           |           - 19ms
 *       20ms -       |           |           |
 *            |       x 21ms      |           |
 *            |       |           x 22ms      |
 *            |       |           |           x 23ms
 *       24ms -       |           |           |
 *            |       - 25ms      |           |
 *            |                   - 26ms      |
 *            |                               - 27ms
 *       28ms -
 *            |
 *            v
 */
func TestAttributesUnaligned(t *testing.T) {
	testCases := []struct {
		name     string
		offset   float32
		expected [][]float32
	}{
		{
			name:     "offset: 0.5",
			offset:   0.5,
			expected: [][]float32{{-2.375}, {0.625}, {-0.875}},
		},
		{
			name:     "offset: 1.0",
			offset:   1.0,
			expected: [][]float32{{-2.250}, {0.750}, {-0.750}},
		},
		{
			name:     "offset: 2.0",
			offset:   2.0,
			expected: [][]float32{{-2.000}, {1.000}, {-0.500}},
		},
		{
			name:     "offset: 3.0",
			offset:   3.0,
			expected: [][]float32{{-1.750}, {1.250}, {-0.250}},
		},
		{
			name:     "offset: 3.5",
			offset:   3.5,
			expected: [][]float32{{-1.625}, {1.375}, {-0.125}},
		},
	}

	const above = float32(8)
	const below = float32(4)
	const stepsize = float32(4)

	targetAttributes := []string{"min", "max", "mean"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	for _, testCase := range testCases {
		values := [][]float32{{20 + testCase.offset}}

		surface := samples10Surface(values)

		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		buf, err := handle.GetAttributesAlongSurface(
			surface,
			above,
			below,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)
		require.NoErrorf(t, err,
			"[%s] Failed to fetch horizon, err: %v", testCase.name, err,
		)
		require.Len(t, buf, len(targetAttributes),
			"Incorrect number of attributes returned",
		)

		for i, attr := range buf {
			result, err := toFloat32(attr)
			require.NoErrorf(t, err, "Couldn't convert to float32")

			require.InDeltaSlicef(
				t,
				testCase.expected[i],
				*result,
				0.000001,
				"[%s][%s]\nExpected: %v\nActual:   %v",
				testCase.name,
				targetAttributes[i],
				testCase.expected[i],
				*result,
			)
		}
	}
}

func TestAttributesSupersampling(t *testing.T) {
	expected := [][]float32{
		{1.000},  //samplevalue
		{-0.250}, // min
		{2.250},  // max
		{1.000},  // mean
	}
	const above = float32(8)
	const below = float32(8)
	const stepsize = float32(5) // VDS is sampled at 4ms

	targetAttributes := []string{"samplevalue", "min", "max", "mean"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	values := [][]float32{{26}}

	surface := samples10Surface(values)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err, "Failed to fetch horizon, err: %v", err)
	require.Len(t, buf, len(targetAttributes),
		"Incorrect number of attributes returned",
	)

	for i, attr := range buf {
		result, err := toFloat32(attr)
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.InDeltaSlicef(
			t,
			expected[i],
			*result,
			0.000001,
			"[%s]\nExpected: %v\nActual:   %v",
			targetAttributes[i],
			expected[i],
			*result,
		)
	}
}

func TestAttributesEverythingUnaligned(t *testing.T) {
	expected := [][]float32{
		{1.000}, //samplevalue
		{0.250}, // min
		{2.500}, // max
		{1.375}, // mean
	}
	const above = float32(5)
	const below = float32(7)
	const stepsize = float32(3)

	targetAttributes := []string{"samplevalue", "min", "max", "mean"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	values := [][]float32{{26}}

	surface := samples10Surface(values)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	buf, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err, "Failed to fetch horizon, err: %v", err)
	require.Len(t, buf, len(targetAttributes),
		"Incorrect number of attributes returned",
	)

	for i, attr := range buf {
		result, err := toFloat32(attr)
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.InDeltaSlicef(
			t,
			expected[i],
			*result,
			0.000001,
			"[%s]\nExpected: %v\nActual:   %v",
			targetAttributes[i],
			expected[i],
			*result,
		)
	}
}

func TestInvalidAboveBelow(t *testing.T) {
	testcases := []struct {
		name  string
		above float32
		below float32
	}{
		{
			name:  "Bad above",
			above: -4,
			below: 1.11,
		},
		{
			name:  "Bad below",
			above: 0,
			below: -6.66,
		},
	}

	targetAttributes := []string{"min", "samplevalue"}
	values := [][]float32{{26}}
	const stepsize = float32(4.0)

	for _, testcase := range testcases {
		surface := samples10Surface(values)
		interpolationMethod, _ := GetInterpolationMethod("nearest")
		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		_, boundsErr := handle.GetAttributesAlongSurface(
			surface,
			testcase.above,
			testcase.below,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)

		require.ErrorContainsf(t, boundsErr,
			"Above and below must be positive",
			"[%s] Expected above/below %f/%f to throw invalid argument",
			testcase.name,
			testcase.above,
			testcase.below,
		)
	}
}

func TestAttributeMetadata(t *testing.T) {
	values := [][]float32{
		{10, 10, 10, 10, 10, 10},
		{10, 10, 10, 10, 10, 10},
	}
	expected := AttributeMetadata{
		Array{
			Format: "<f4",
			Shape:  []int{2, 6},
		},
	}

	handle, _ := NewDSHandle(well_known)
	defer handle.Close()
	buf, err := handle.GetAttributeMetadata(values)
	require.NoErrorf(t, err, "Failed to retrieve attribute metadata, err %v", err)

	var meta AttributeMetadata
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	err = dec.Decode(&meta)
	require.NoErrorf(t, err, "Failed to unmarshall response, err: %v", err)

	require.Equal(t, expected, meta)
}

func TestAttributeBetweenSurfaces(t *testing.T) {
	expected := map[string][]float32{
		"samplevalue": {},
		"min":         {-1.5, -0.5, -8.5, 5.5, fillValue, -24.5, fillValue, fillValue},
		"min_at":      {16, 24, 20, 18, fillValue, 28, fillValue, fillValue},
		"max":         {2.5, 0.5, -8.5, 5.5, fillValue, -8.5, fillValue, fillValue},
		"max_at":      {32, 20, 20, 18, fillValue, 12, fillValue, fillValue},
		"maxabs":      {2.5, 0.5, 8.5, 5.5, fillValue, 24.5, fillValue, fillValue},
		"maxabs_at":   {32, 20, 20, 18, fillValue, 28, fillValue, fillValue},
		"mean":        {0.5, 0, -8.5, 5.5, fillValue, -16.5, fillValue, fillValue},
		"meanabs":     {1.3, 0.5, 8.5, 5.5, fillValue, 16.5, fillValue, fillValue},
		"meanpos":     {1.5, 0.5, 0, 5.5, fillValue, 0, fillValue, fillValue},
		"meanneg":     {-1, -0.5, -8.5, 0, fillValue, -16.5, fillValue, fillValue},
		"median":      {0.5, 0, -8.5, 5.5, fillValue, -16.5, fillValue, fillValue},
		"rms":         {1.5, 0.5, 8.5, 5.5, fillValue, 17.442764, fillValue, fillValue},
		"var":         {2, 0.25, 0, 0, fillValue, 32, fillValue, fillValue},
		"sd":          {1.4142135, 0.5, 0, 0, fillValue, 5.656854, fillValue, fillValue},
		"sumpos":      {4.5, 0.5, 0, 5.5, fillValue, 0, fillValue, fillValue},
		"sumneg":      {-2, -0.5, -8.5, 0, fillValue, -82.5, fillValue, fillValue},
	}

	targetAttributes := make([]string, 0, len(expected))
	for attr := range expected {
		targetAttributes = append(targetAttributes, attr)
	}
	sort.Strings(targetAttributes)

	topValues := [][]float32{
		{16, 20},
		{20, 18},
		{14, 12},
		{12, 12}, // Out-of-bounds
	}
	bottomValues := [][]float32{
		{32, 24},
		{20, 18},
		{fillValue, 28},
		{28, 28}, // Out-of-bounds
	}
	const stepsize = float32(4.0)

	topSurface := samples10Surface(topValues)
	bottomSurface := samples10Surface(bottomValues)

	testcases := []struct {
		primary             RegularSurface
		secondary           RegularSurface
		expectedSamplevalue []float32
	}{
		{
			primary:             topSurface,
			secondary:           bottomSurface,
			expectedSamplevalue: []float32{-1.5, 0.5, -8.5, 5.5, fillValue, -8.5, fillValue, fillValue},
		},
		{
			primary:             bottomSurface,
			secondary:           topSurface,
			expectedSamplevalue: []float32{2.5, -0.5, -8.5, 5.5, fillValue, -24.5, fillValue, fillValue},
		},
	}

	interpolationMethod, _ := GetInterpolationMethod("nearest")

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()

	for _, testcase := range testcases {
		expected["samplevalue"] = testcase.expectedSamplevalue
		buf, err := handle.GetAttributesBetweenSurfaces(
			testcase.primary,
			testcase.secondary,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)
		require.NoErrorf(t, err, "Failed to calculate attributes, err %v", err)
		require.Len(t, buf, len(targetAttributes),
			"Incorrect number of attributes returned",
		)

		for i, attr := range buf {
			result, err := toFloat32(attr)
			require.NoErrorf(t, err, "Couldn't convert to float32")

			require.InDeltaSlicef(
				t,
				expected[targetAttributes[i]],
				*result,
				0.000001,
				"[%s]\nExpected: %v\nActual:   %v",
				targetAttributes[i],
				expected[targetAttributes[i]],
				*result,
			)
		}
	}
}

func TestAttributesInconsistentLength(t *testing.T) {
	const above = float32(0)
	const below = float32(0)
	const stepsize = float32(4)
	targetAttributes := []string{"samplevalue"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	goodValues := [][]float32{{20, 20, 20}, {20, 20, 20}}
	badValues := [][]float32{{20, 20}, {20, 20, 20}}

	errmsg := "Surface rows are not of the same length. " +
		"Row 0 has 2 elements. Row 1 has 3 elements"

	goodSurface := samples10Surface(goodValues)
	badSurface := samples10Surface(badValues)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()

	_, err := handle.GetAttributesAlongSurface(
		badSurface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.ErrorContains(t, err, errmsg, err)

	_, err = handle.GetAttributesBetweenSurfaces(
		badSurface,
		goodSurface,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.ErrorContains(t, err, errmsg, err)

	_, err = handle.GetAttributesBetweenSurfaces(
		goodSurface,
		badSurface,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.ErrorContains(t, err, errmsg, err)
}

func TestAttributesAllFill(t *testing.T) {
	const above = float32(0)
	const below = float32(0)
	const stepsize = float32(4)
	targetAttributes := []string{"samplevalue", "min"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	fillValues := [][]float32{{fillValue, fillValue, fillValue}, {fillValue, fillValue, fillValue}}
	fillSurface := samples10Surface(fillValues)
	expected := []float32{fillValue, fillValue, fillValue, fillValue, fillValue, fillValue}

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()

	bufAlong, err := handle.GetAttributesAlongSurface(
		fillSurface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err,
		"Along: Failed to calculate attributes, err: %v",
		err,
	)

	bufBetween, err := handle.GetAttributesBetweenSurfaces(
		fillSurface,
		fillSurface,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err,
		"Between: Failed to calculate attributes, err: %v",
		err,
	)

	require.Len(t, bufAlong, len(targetAttributes), "Along: Wrong number of attributes")
	require.Len(t, bufBetween, len(targetAttributes), "Between: Wrong number of attributes")

	for i, attr := range targetAttributes {
		along, err := toFloat32(bufAlong[i])
		require.NoErrorf(t, err, "Couldn't convert to float32")
		between, err := toFloat32(bufBetween[i])
		require.NoErrorf(t, err, "Couldn't convert to float32")

		require.Equalf(t, expected, *along, "[%v]", attr)
		require.Equalf(t, expected, *between, "[%v]", attr)
	}
}

func TestAttributesOnSamplesNearTraceBoundary(t *testing.T) {
	const stepsize = float32(4)
	targetAttributes := []string{"mean"}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	topSurfaceValues := [][]float32{{8, 4}, {4, 32}, {36, 36}}
	bottomSurfaceValues := [][]float32{{12, 12}, {8, 36}, {40, 40}}

	topSurface := samples10Surface(topSurfaceValues)
	bottomSurface := samples10Surface(bottomSurfaceValues)

	expected := []float32{-3, 3.5, 5.5, 13.5, 12.5, -12.5}

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()

	buf, err := handle.GetAttributesBetweenSurfaces(
		topSurface,
		bottomSurface,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.NoErrorf(t, err,
		"Failed to calculate attributes, err: %v",
		err,
	)

	require.Len(t, buf, len(targetAttributes), "Wrong number of attributes")

	for i, attr := range targetAttributes {
		value, err := toFloat32(buf[i])
		require.NoErrorf(t, err, "Couldn't convert to float32")
		require.Equalf(t, expected, *value, "[%v]", attr)
	}
}

func TestAttributesInsufficientInterpolationSamples(t *testing.T) {
	testcases := []struct {
		name        string
		top         [][]float32
		bottom      [][]float32
		expectedMin [][]float32
		expectedMax [][]float32
	}{
		// top survey boundary at 4.0, bottom boundary at 40.0
		{
			name:        "No samples between top and bottom border in the middle",
			top:         [][]float32{{22.00}},
			bottom:      [][]float32{{23.00}},
			expectedMin: [][]float32{{0.00}},
			expectedMax: [][]float32{{0.25}},
		},
		{
			name:        "No samples between top and bottom border, two to the top survey boundary",
			top:         [][]float32{{10.00}},
			bottom:      [][]float32{{11.00}},
			expectedMin: [][]float32{{-3.00}},
			expectedMax: [][]float32{{-2.75}},
		},
		{
			name:        "No samples between top and bottom border, one to the top survey boundary",
			top:         [][]float32{{6.00}},
			bottom:      [][]float32{{7.00}},
			expectedMin: [][]float32{{-4.00}},
			expectedMax: [][]float32{{-3.75}},
		},
		{
			name:        "One sample between top and bottom border, one to the top survey boundary",
			top:         [][]float32{{6.00}},
			bottom:      [][]float32{{11.00}},
			expectedMin: [][]float32{{-4.00}},
			expectedMax: [][]float32{{-2.75}},
		},
		{
			name:        "Top boundary on the top survey boundary, no additional samples between borders",
			top:         [][]float32{{4.00}},
			bottom:      [][]float32{{7.00}},
			expectedMin: [][]float32{{-4.50}},
			expectedMax: [][]float32{{-3.75}},
		},
		{
			name:        "Top boundary on the top survey boundary + additional sample between borders",
			top:         [][]float32{{4.00}},
			bottom:      [][]float32{{11.00}},
			expectedMin: [][]float32{{-4.50}},
			expectedMax: [][]float32{{-2.75}},
		},
		{
			name:        "No samples between top and bottom border, one to the bottom survey boundary",
			top:         [][]float32{{36.50}},
			bottom:      [][]float32{{39.00}},
			expectedMin: [][]float32{{3.625}},
			expectedMax: [][]float32{{4.25}},
		},
		{
			name:        "One sample between top and bottom border, one to the bottom survey boundary",
			top:         [][]float32{{35.50}},
			bottom:      [][]float32{{39.00}},
			expectedMin: [][]float32{{3.375}},
			expectedMax: [][]float32{{4.25}},
		},
		{
			name:        "Bottom boundary on the bottom survey boundary, no additional samples between borders",
			top:         [][]float32{{39.50}},
			bottom:      [][]float32{{40.00}},
			expectedMin: [][]float32{{4.375}},
			expectedMax: [][]float32{{4.5}},
		},
	}

	targetAttributes := []string{"min_at", "min", "max_at", "max"}
	const stepsize = float32(0.5)
	interpolationMethod, _ := GetInterpolationMethod("linear")

	for _, testcase := range testcases {
		topSurface := samples10Surface(testcase.top)
		bottomSurface := samples10Surface(testcase.bottom)

		handle, _ := NewDSHandle(samples10)
		defer handle.Close()
		buf, boundsErr := handle.GetAttributesBetweenSurfaces(
			topSurface,
			bottomSurface,
			stepsize,
			targetAttributes,
			interpolationMethod,
		)

		require.NoErrorf(t, boundsErr,
			"[%s] Expected enough samples between top %v and bottom %v",
			testcase.name,
			testcase.top,
			testcase.bottom,
		)

		require.Len(t, buf, len(targetAttributes), "Wrong number of attributes")

		checkAttributes := func(buf [][]byte, attrn int, expected [][]float32, label string) {
			for i, attr := range buf[len(buf)/4*attrn : len(buf)/4*(attrn+1)] {
				result, err := toFloat32(attr)
				require.NoErrorf(t, err, "Couldn't convert to float32")

				require.Equalf(t, expected[i], *result, "[%v]: Wrong %s values", testcase.name, label)
			}
		}
		expected := [][][]float32{testcase.top, testcase.expectedMin, testcase.bottom, testcase.expectedMax}
		for i, attr := range targetAttributes {
			checkAttributes(buf, i, expected[i], attr)
		}
	}
}

func TestAttributesEmptyArrayParams(t *testing.T) {
	const above = float32(0)
	const below = float32(0)
	const stepsize = float32(4)
	targetAttributes := []string{}
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	values := [][]float32{{}}
	surface := samples10Surface(values)

	goodValues := [][]float32{{20}}
	goodSurface := samples10Surface(goodValues)

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()

	_, err := handle.GetAttributesAlongSurface(
		surface,
		above,
		below,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.Errorf(t, err,
		"Empty surface values didn't throw, err: %v",
		err,
	)
	require.ErrorContainsf(t, err,
		"Surface should contain at least one value",
		"Wrong error for empty surface values",
	)

	_, err = handle.GetAttributesBetweenSurfaces(
		goodSurface,
		goodSurface,
		stepsize,
		targetAttributes,
		interpolationMethod,
	)
	require.Errorf(t, err,
		"Empty surface attributes didn't throw, err: %v",
		err,
	)
	require.ErrorContainsf(t, err,
		"Attributes should contain at least one value",
		"Wrong error for empty surface attributes",
	)
}
