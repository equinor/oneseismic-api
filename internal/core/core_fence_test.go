package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFence(t *testing.T) {
	expected := []float32{
		108, 109, 110, 111, // il: 3, xl: 10, samples: all
		112, 113, 114, 115, // il: 3, xl: 11, samples: all
		100, 101, 102, 103, // il: 1, xl: 10, samples: all
		108, 109, 110, 111, // il: 3, xl: 10, samples: all
		116, 117, 118, 119, // il: 5, xl: 10, samples: all
	}

	testcases := []struct {
		coordinate_system int
		coordinates       [][]float32
	}{
		{
			coordinate_system: CoordinateSystemIndex,
			coordinates:       [][]float32{{1, 0}, {1, 1}, {0, 0}, {1, 0}, {2, 0}},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{3, 10}, {3, 11}, {1, 10}, {3, 10}, {5, 10}},
		},
		{
			coordinate_system: CoordinateSystemCdp,
			coordinates:       [][]float32{{8, 4}, {6, 7}, {2, 0}, {8, 4}, {14, 8}},
		},
	}
	var fillValue float32 = -999.25
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	for _, testcase := range testcases {
		handle, _ := NewDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetFence(
			testcase.coordinate_system,
			testcase.coordinates,
			interpolationMethod,
			&fillValue,
		)
		require.NoErrorf(t, err,
			"[coordinate_system: %v] Failed to fetch fence, err: %v",
			testcase.coordinate_system, err,
		)

		fence, err := toFloat32(buf)
		require.NoErrorf(t, err,
			"[coordinate_system: %v] Err: %v", testcase.coordinate_system, err,
		)

		require.Equalf(t, expected, *fence, "Incorrect fence")
	}
}

func TestFenceBorders(t *testing.T) {
	testcases := []struct {
		name              string
		coordinate_system int
		coordinates       [][]float32
		interpolation     string
		err               string
	}{
		{
			name:              "coordinate 1 is just-out-of-upper-bound in direction 0",
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{5, 9.5}, {6, 11.25}},
			err:               "is out of boundaries in dimension 0.",
		},
		{
			name:              "coordinate 0 is just-out-of-upper-bound in direction 1",
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{5.5, 11.5}, {3, 10}},
			err:               "is out of boundaries in dimension 1.",
		},
		{
			name:              "coordinate is long way out of upper-bound in both directions",
			coordinate_system: CoordinateSystemCdp,
			coordinates:       [][]float32{{700, 1200}},
			err:               "is out of boundaries in dimension 0.",
		},
		{
			name:              "coordinate 1 is just-out-of-lower-bound in direction 1",
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{0, 11}, {5.9999, 10}, {0.0001, 9.4999}},
			err:               "is out of boundaries in dimension 1.",
		},
		{
			name:              "negative coordinate 0 is out-of-lower-bound in direction 0",
			coordinate_system: CoordinateSystemIndex,
			coordinates:       [][]float32{{-1, 0}, {-3, 0}},
			err:               "is out of boundaries in dimension 0.",
		},
	}

	for _, testcase := range testcases {
		interpolationMethod, _ := GetInterpolationMethod("linear")
		handle, _ := NewDSHandle(well_known)
		defer handle.Close()
		_, err := handle.GetFence(testcase.coordinate_system, testcase.coordinates, interpolationMethod, nil)

		require.ErrorContainsf(t, err, testcase.err, "[case: %v]", testcase.name)
	}
}

func TestFenceBordersWithFillValue(t *testing.T) {
	testcases := []struct {
		name          string
		crd_system    int
		coordinates   [][]float32
		interpolation string
		fence         []float32
	}{
		{
			name:        "coordinate 1 is just-out-of-upper-bound in direction 0",
			crd_system:  CoordinateSystemAnnotation,
			coordinates: [][]float32{{5, 9.5}, {6, 11.25}},
			fence:       []float32{116, 117, 118, 119, -999.25, -999.25, -999.25, -999.25},
		},
		{
			name:        "coordinate 0 is just-out-of-upper-bound in direction 1",
			crd_system:  CoordinateSystemAnnotation,
			coordinates: [][]float32{{5.5, 11.5}, {3, 10}},
			fence:       []float32{-999.25, -999.25, -999.25, -999.25, 108, 109, 110, 111},
		},
		{
			name:        "coordinate is long way out of upper-bound in both directions",
			crd_system:  CoordinateSystemCdp,
			coordinates: [][]float32{{700, 1200}},
			fence:       []float32{-999.25, -999.25, -999.25, -999.25},
		},
		{
			name:        "coordinate 1 is just-out-of-lower-bound in direction 1",
			crd_system:  CoordinateSystemAnnotation,
			coordinates: [][]float32{{0, 11}, {5.9999, 10}, {0.0001, 9.4999}},
			fence:       []float32{104, 105, 106, 107, 116, 117, 118, 119, -999.25, -999.25, -999.25, -999.25},
		},
		{
			name:        "negative coordinate 0 is out-of-lower-bound in direction 0",
			crd_system:  CoordinateSystemIndex,
			coordinates: [][]float32{{-1, 0}, {-3, 0}},
			fence:       []float32{-999.25, -999.25, -999.25, -999.25, -999.25, -999.25, -999.25, -999.25},
		},
	}

	var fillValue float32 = -999.25
	for _, testcase := range testcases {
		interpolationMethod, _ := GetInterpolationMethod("linear")
		handle, _ := NewDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetFence(
			testcase.crd_system,
			testcase.coordinates,
			interpolationMethod,
			&fillValue,
		)
		require.NoError(t, err)

		fence, err := toFloat32(buf)
		require.NoErrorf(t, err,
			"[coordinate_system: %v] Err: %v", testcase.crd_system, err,
		)
		require.Equal(t, testcase.fence, *fence, "[case: %v]", testcase.name)
	}
}

func TestFenceNearestInterpolationSnap(t *testing.T) {
	testcases := []struct {
		coordinate_system int
		coordinates       [][]float32
		expected          []float32
	}{
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{3, 10}},
			expected:          []float32{108, 109, 110, 111},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{3.5, 10.25}},
			expected:          []float32{108, 109, 110, 111},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{3.9999, 10.4999}},
			expected:          []float32{108, 109, 110, 111},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{4, 10.5}},
			expected:          []float32{120, 121, 122, 123},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{4.5, 10.75}},
			expected:          []float32{120, 121, 122, 123},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{5, 11}},
			expected:          []float32{120, 121, 122, 123},
		},
		{
			coordinate_system: CoordinateSystemAnnotation,
			coordinates:       [][]float32{{5.5, 11.25}},
			expected:          []float32{120, 121, 122, 123},
		},
		{
			coordinate_system: CoordinateSystemIndex,
			coordinates:       [][]float32{{-0.5, -0.5}},
			expected:          []float32{100, 101, 102, 103},
		},
		{
			coordinate_system: CoordinateSystemCdp,
			coordinates:       [][]float32{{10, 5}},
			expected:          []float32{108, 109, 110, 111},
		},
		{
			coordinate_system: CoordinateSystemCdp,
			coordinates:       [][]float32{{10, 7.5}},
			expected:          []float32{120, 121, 122, 123},
		},
	}
	var fillValue float32 = -999.25
	interpolationMethod, _ := GetInterpolationMethod("nearest")

	for _, testcase := range testcases {
		handle, _ := NewDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetFence(
			testcase.coordinate_system,
			testcase.coordinates,
			interpolationMethod,
			&fillValue,
		)
		require.NoErrorf(t, err,
			"[coordinate_system: %v] Failed to fetch fence, err: %v",
			testcase.coordinate_system,
			err,
		)

		fence, err := toFloat32(buf)
		require.NoErrorf(t, err,
			"[coordinate_system: %v] Err: %v", testcase.coordinate_system, err,
		)

		require.Equalf(t, testcase.expected, *fence, "Incorrect fence")
	}
}

func TestInvalidFence(t *testing.T) {
	fence := [][]float32{{1, 0}, {1, 1, 0}, {0, 0}, {1, 0}, {2, 0}}

	var fillValue float32 = -999.25
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	handle, _ := NewDSHandle(well_known)
	defer handle.Close()
	_, err := handle.GetFence(CoordinateSystemIndex, fence, interpolationMethod, &fillValue)

	require.ErrorContains(t, err,
		"invalid coordinate [1 1 0] at position 1, expected [x y] pair",
	)
}

// As interpolation algorithms are complicated and allow some variety, it would
// not be feasible to manually try to figure out expected value for each
// interpolation method. So we have to trust openvds on this.
//
// But we can check that we use openvds correctly by checking interpolated data
// at known data points. Openvds claims that data would still be the same for all
// interpolation algorithms [1].
//
// We had a bug that caused cubic and linear return incorrect values. So this is
// the best feasible test that would help us guard against that bug.
//
// [1] angular is skipped on purpose as it anyway will hold only for files where
// ValueRange is defined correctly
// https://community.opengroup.org/osdu/platform/domain-data-mgmt-services/seismic/open-vds/-/issues/171.
func TestFenceInterpolationSameAtDataPoints(t *testing.T) {
	// use non-linear data
	coordinates := [][]float32{{2, 0}}
	expected := []float32{25.5, 4.5, 8.5, 12.5, 16.5, 20.5, 24.5, 20.5, 16.5, 8.5}
	interpolationMethods := []string{"nearest", "linear", "cubic", "triangular"}

	handle, _ := NewDSHandle(samples10)
	defer handle.Close()
	var fillValue float32 = -999.25
	for _, interpolation := range interpolationMethods {
		interpolationMethod, _ := GetInterpolationMethod(interpolation)
		buf, err := handle.GetFence(
			CoordinateSystemIndex,
			coordinates,
			interpolationMethod,
			&fillValue,
		)
		require.NoErrorf(t, err, "Failed to fetch fence in [interpolation: %v]", interpolation)
		result, err := toFloat32(buf)
		require.NoErrorf(t, err, "Failed to covert to float32 buffer [interpolation: %v]", interpolation)
		require.Equalf(t, expected, *result, "Fence not as expected [interpolation: %v]", interpolation)
	}
}

// Also, as we can't check interpolation properly, check that beyond data points
// different values are returned by each algorithm
func TestFenceInterpolationDifferentBeyondDatapoints(t *testing.T) {
	fence := [][]float32{{3.2, 3}, {3.2, 6.3}, {1, 3}, {3.2, 3}, {5.4, 3}}
	var fillValue float32 = -999.25
	interpolationMethods := []string{"nearest", "linear", "cubic", "triangular", "angular"}
	for i, v1 := range interpolationMethods {
		interpolationMethod, _ := GetInterpolationMethod(v1)
		handle, _ := NewDSHandle(well_known)
		defer handle.Close()
		buf1, _ := handle.GetFence(CoordinateSystemCdp, fence, interpolationMethod, &fillValue)
		for _, v2 := range interpolationMethods[i+1:] {
			interpolationMethod, _ := GetInterpolationMethod(v2)
			buf2, _ := handle.GetFence(CoordinateSystemCdp, fence, interpolationMethod, &fillValue)

			require.NotEqual(t, buf1, buf2)
		}
	}
}

func TestFenceInterpolationDefaultIsNearest(t *testing.T) {
	defaultInterpolation, _ := GetInterpolationMethod("")
	nearestInterpolation, _ := GetInterpolationMethod("nearest")

	require.Equalf(t, defaultInterpolation, nearestInterpolation, "Default interpolation is not nearest")
}

func TestInvalidInterpolationMethod(t *testing.T) {
	options := "nearest, linear, cubic, angular or triangular"
	expected := NewInvalidArgument(fmt.Sprintf(
		"invalid interpolation method 'sand', valid options are: %s",
		options,
	))

	interpolation := "sand"
	_, err := GetInterpolationMethod(interpolation)

	require.Equal(t, err, expected)
}

func TestFenceInterpolationCaseInsensitive(t *testing.T) {
	expectedInterpolation, _ := GetInterpolationMethod("cubic")
	interpolation, _ := GetInterpolationMethod("CuBiC")

	require.Equal(t, interpolation, expectedInterpolation)
}

func TestFenceMetadata(t *testing.T) {
	coordinates := [][]float32{{5, 10}, {5, 10}, {1, 11}, {2, 11}, {4, 11}}
	expected := FenceMetadata{
		Array{
			Format: "<f4",
			Shape:  []int{5, 4},
		},
	}

	handle, _ := NewDSHandle(well_known)
	defer handle.Close()
	buf, err := handle.GetFenceMetadata(coordinates)
	require.NoErrorf(t, err, "Failed to retrieve fence metadata, err %v", err)

	var meta FenceMetadata
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	err = dec.Decode(&meta)
	require.NoErrorf(t, err, "Failed to unmarshall response, err: %v", err)

	require.Equal(t, expected, meta)
}

func TestFenceEmptyArrayParams(t *testing.T) {
	fence := [][]float32{}

	var fillValue float32 = -999.25
	interpolationMethod, _ := GetInterpolationMethod("nearest")
	handle, _ := NewDSHandle(well_known)
	defer handle.Close()
	_, err := handle.GetFence(CoordinateSystemIndex, fence, interpolationMethod, &fillValue)

	require.Errorf(t, err,
		"Empty coordinates didn't throw, err: %v",
		err,
	)
	require.ErrorContainsf(t, err,
		"Coordinates should contain at least one value",
		"Wrong error for empty coordinates",
	)
}
