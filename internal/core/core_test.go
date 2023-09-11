package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func make_connection(name string) Connection {
	path := fmt.Sprintf("../../testdata/%s/%s_default.vds", name, name)
	path = fmt.Sprintf("file://%s", path)
	return NewFileConnection(path)
}

var well_known = make_connection("well_known")
var samples10 = make_connection("10_samples")
var prestack = make_connection("prestack")

var fillValue = float32(-999.25)

type gridDefinition struct {
	xori     float32
	yori     float32
	xinc     float32
	yinc     float32
	rotation float32
}

/* The grid definition for the well_known vds test file
 *
 * When constructing horizons for testing it's useful to reuse the same grid
 * definition in the horizon as in the vds itself.
 */
var well_known_grid gridDefinition = gridDefinition{
	xori:     float32(2.0),
	yori:     float32(0.0),
	xinc:     float32(7.2111),
	yinc:     float32(3.6056),
	rotation: float32(33.69),
}

var samples10_grid = well_known_grid

func samples10Surface(data [][]float32) RegularSurface {
	return RegularSurface{
		Values:    data,
		Rotation:  &samples10_grid.rotation,
		Xori:      &samples10_grid.xori,
		Yori:      &samples10_grid.yori,
		Xinc:      samples10_grid.xinc,
		Yinc:      samples10_grid.yinc,
		FillValue: &fillValue,
	}
}

func toFloat32(buf []byte) (*[]float32, error) {
	fsize := 4 // sizeof(float32)
	if len(buf)%fsize != 0 {
		return nil, errors.New("Invalid buffersize")
	}

	outlen := len(buf) / 4

	out := make([]float32, outlen)
	for i := 0; i < outlen; i++ {
		offset := i * fsize
		tmp := binary.LittleEndian.Uint32(buf[offset : offset+fsize])
		out[i] = math.Float32frombits(tmp)
	}

	return &out, nil
}

func TestMetadata(t *testing.T) {
	expected := Metadata{
		Axis: []*Axis{
			{Annotation: "Inline", Min: 1, Max: 5, Samples: 3, StepSize: 2, Unit: "unitless"},
			{Annotation: "Crossline", Min: 10, Max: 11, Samples: 2, StepSize: 1, Unit: "unitless"},
			{Annotation: "Sample", Min: 4, Max: 16, Samples: 4, StepSize: 4, Unit: "ms"},
		},
		BoundingBox: BoundingBox{
			Cdp:  [][]float64{{2, 0}, {14, 8}, {12, 11}, {0, 3}},
			Ilxl: [][]float64{{1, 10}, {5, 10}, {5, 11}, {1, 11}},
			Ij:   [][]float64{{0, 0}, {2, 0}, {2, 1}, {0, 1}},
		},
		Crs:             "utmXX",
		InputFileName:   "well_known.segy",
		ImportTimeStamp: `^\d{4}-\d{2}-\d{2}[A-Z]\d{2}:\d{2}:\d{2}\.\d{3}[A-Z]$`,
	}

	handle, _ := NewVDSHandle(well_known)
	defer handle.Close()
	buf, err := handle.GetMetadata()
	require.NoErrorf(t, err, "Failed to retrive metadata, err %v", err)

	var meta Metadata
	err = json.Unmarshal(buf, &meta)
	require.NoErrorf(t, err, "Failed to unmarshall response, err: %v", err)

	require.Regexp(t, expected.ImportTimeStamp, meta.ImportTimeStamp)

	expected.ImportTimeStamp = "dummy"
	meta.ImportTimeStamp = "dummy"

	require.Equal(t, meta, expected)
}

func TestSliceData(t *testing.T) {
	il := []float32{
		108, 109, 110, 111, // il: 3, xl: 10, samples: all
		112, 113, 114, 115, // il: 3, xl: 11, samples: all
	}

	xl := []float32{
		100, 101, 102, 103, // il: 1, xl: 10, samples: all
		108, 109, 110, 111, // il: 3, xl: 10, samples: all
		116, 117, 118, 119, // il: 5, xl: 10, samples: all
	}

	time := []float32{
		101, 105, // il: 1, xl: all, samples: 1
		109, 113, // il: 3, xl: all, samples: 1
		117, 121, // il: 5, xl: all, samples: 1
	}

	testcases := []struct {
		name      string
		lineno    int
		direction int
		expected  []float32
	}{
		{name: "inline", lineno: 3, direction: AxisInline, expected: il},
		{name: "i", lineno: 1, direction: AxisI, expected: il},
		{name: "crossline", lineno: 10, direction: AxisCrossline, expected: xl},
		{name: "j", lineno: 0, direction: AxisJ, expected: xl},
		{name: "time", lineno: 8, direction: AxisTime, expected: time},
		{name: "j", lineno: 1, direction: AxisK, expected: time},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetSlice(
			testcase.lineno,
			testcase.direction,
			[]Bound{},
		)
		require.NoErrorf(t, err,
			"[case: %v] Failed to fetch slice, err: %v",
			testcase.name,
			err,
		)

		slice, err := toFloat32(buf)
		require.NoErrorf(t, err, "[case: %v] Err: %v", testcase.name, err)

		require.Equalf(t, testcase.expected, *slice, "[case: %v]", testcase.name)
	}
}

func TestSliceOutOfBounds(t *testing.T) {
	testcases := []struct {
		name      string
		lineno    int
		direction int
	}{
		{name: "Inline too low", lineno: 0, direction: AxisInline},
		{name: "Inline too high", lineno: 6, direction: AxisInline},
		{name: "Crossline too low", lineno: 9, direction: AxisCrossline},
		{name: "Crossline too high", lineno: 12, direction: AxisCrossline},
		{name: "Time too low", lineno: 3, direction: AxisTime},
		{name: "Time too high", lineno: 17, direction: AxisTime},
		{name: "I too low", lineno: -1, direction: AxisI},
		{name: "I too high", lineno: 3, direction: AxisI},
		{name: "J too low", lineno: -1, direction: AxisJ},
		{name: "J too high", lineno: 2, direction: AxisJ},
		{name: "K too low", lineno: -1, direction: AxisK},
		{name: "K too high", lineno: 4, direction: AxisK},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		_, err := handle.GetSlice(
			testcase.lineno,
			testcase.direction,
			[]Bound{},
		)

		require.ErrorContains(t, err, "Invalid lineno")
	}

}

func TestSliceStepSizedLineno(t *testing.T) {
	testcases := []struct {
		name      string
		lineno    int
		direction int
	}{
		{name: "inline", lineno: 2, direction: AxisInline},
		{name: "time", lineno: 5, direction: AxisTime},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		_, err := handle.GetSlice(
			testcase.lineno,
			testcase.direction,
			[]Bound{},
		)

		require.ErrorContains(t, err, "Invalid lineno")
	}
}

func TestSliceInvalidAxis(t *testing.T) {
	testcases := []struct {
		name      string
		direction int
	}{
		{name: "Too small", direction: -1},
		{name: "Too large", direction: 8},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		_, err := handle.GetSlice(0, testcase.direction, []Bound{})

		require.ErrorContains(t, err, "Unhandled axis")
	}
}

func TestSliceBounds(t *testing.T) {
	newBound := func(direction string, lower, upper int) Bound {
		return Bound{Direction: &direction, Lower: &lower, Upper: &upper}
	}
	newAxis := func(
		annotation string,
		min float64,
		max float64,
		samples int,
		stepsize float64,
	) Axis {
		unit := "ms"
		anno := strings.ToLower(annotation)
		if anno == "inline" || anno == "crossline" {
			unit = "unitless"
		}

		return Axis{
			Annotation: annotation,
			Min:        min,
			Max:        max,
			Samples:    samples,
			StepSize:   stepsize,
			Unit:       unit,
		}
	}

	testCases := []struct {
		name          string
		direction     string
		lineno        int
		bounds        []Bound
		expectedSlice []float32
		expectedErr   error
		expectedShape []int
		expectedXAxis Axis
		expectedYAxis Axis
		expectedGeo   [][]float64
	}{
		{
			name:      "Constraint in slice dir is ignored - same coordinate system",
			direction: "i",
			lineno:    1,
			bounds: []Bound{
				newBound("i", 0, 1),
			},
			expectedSlice: []float32{
				108, 109, 110, 111,
				112, 113, 114, 115,
			},
			expectedShape: []int{2, 4},
			expectedXAxis: newAxis("Sample", 4, 16, 4, 4),
			expectedYAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedGeo:   [][]float64{{8, 4}, {6, 7}},
		},
		{
			name:      "Constraint in slice dir is ignored - different coordinate system",
			direction: "crossline",
			lineno:    10,
			bounds: []Bound{
				newBound("j", 0, 1),
			},
			expectedSlice: []float32{
				100, 101, 102, 103,
				108, 109, 110, 111,
				116, 117, 118, 119,
			},
			expectedShape: []int{3, 4},
			expectedXAxis: newAxis("Sample", 4, 16, 4, 4),
			expectedYAxis: newAxis("Inline", 1, 5, 3, 2),
			expectedGeo:   [][]float64{{2, 0}, {14, 8}},
		},
		{
			name:      "Multiple constraints in slice dir are all ignored",
			direction: "time",
			lineno:    4,
			bounds: []Bound{
				newBound("time", 8, 12),
				newBound("k", 0, 1),
			},
			expectedSlice: []float32{
				100, 104,
				108, 112,
				116, 120,
			},
			expectedShape: []int{3, 2},
			expectedXAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedYAxis: newAxis("Inline", 1, 5, 3, 2),
			expectedGeo:   [][]float64{{2, 0}, {14, 8}, {12, 11}, {0, 3}},
		},
		{
			name:      "Single constraint - same coordinate system",
			direction: "inline",
			lineno:    3,
			bounds: []Bound{
				newBound("crossline", 10, 10),
			},
			expectedSlice: []float32{
				108, 109, 110, 111,
			},
			expectedShape: []int{1, 4},
			expectedXAxis: newAxis("Sample", 4, 16, 4, 4),
			expectedYAxis: newAxis("Crossline", 10, 10, 1, 1),
			expectedGeo:   [][]float64{{8, 4}, {8, 4}},
		},
		{
			name:      "Single constraint - different coordinate system",
			direction: "sample",
			lineno:    4,
			bounds: []Bound{
				newBound("i", 0, 1),
			},
			expectedSlice: []float32{
				100, 104,
				108, 112,
			},
			expectedShape: []int{2, 2},
			expectedXAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedYAxis: newAxis("Inline", 1, 3, 2, 2),
			expectedGeo:   [][]float64{{2, 0}, {8, 4}, {6, 7}, {0, 3}},
		},
		{
			name:      "Two constraints - same coordinate system",
			direction: "k",
			lineno:    0,
			bounds: []Bound{
				newBound("i", 0, 1),
				newBound("j", 0, 0),
			},
			expectedSlice: []float32{
				100,
				108,
			},
			expectedShape: []int{2, 1},
			expectedXAxis: newAxis("Crossline", 10, 10, 1, 1),
			expectedYAxis: newAxis("Inline", 1, 3, 2, 2),
			expectedGeo:   [][]float64{{2, 0}, {8, 4}, {8, 4}, {2, 0}},
		},
		{
			name:      "Two constraints - different coordinate system",
			direction: "j",
			lineno:    0,
			bounds: []Bound{
				newBound("inline", 1, 3),
				newBound("k", 1, 2),
			},
			expectedSlice: []float32{
				101, 102,
				109, 110,
			},
			expectedShape: []int{2, 2},
			expectedXAxis: newAxis("Sample", 8, 12, 2, 4),
			expectedYAxis: newAxis("Inline", 1, 3, 2, 2),
			expectedGeo:   [][]float64{{2, 0}, {8, 4}},
		},
		{
			name:      "Horizonal bounds for full axis range is the same as no bound",
			direction: "time",
			lineno:    12,
			bounds: []Bound{
				newBound("crossline", 10, 11),
				newBound("i", 0, 2),
			},
			expectedSlice: []float32{
				102, 106,
				110, 114,
				118, 122,
			},
			expectedShape: []int{3, 2},
			expectedXAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedYAxis: newAxis("Inline", 1, 5, 3, 2),
			expectedGeo:   [][]float64{{2, 0}, {14, 8}, {12, 11}, {0, 3}},
		},
		{
			name:      "Vertical bounds for full axis range is the same as no bound",
			direction: "inline",
			lineno:    5,
			bounds: []Bound{
				newBound("time", 4, 16),
			},
			expectedSlice: []float32{
				116, 117, 118, 119,
				120, 121, 122, 123,
			},
			expectedShape: []int{2, 4},
			expectedXAxis: newAxis("Sample", 4, 16, 4, 4),
			expectedYAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedGeo:   [][]float64{{14, 8}, {12, 11}},
		},
		{
			name:      "The last bound takes precedence",
			direction: "inline",
			lineno:    5,
			bounds: []Bound{
				newBound("time", 4, 8),
				newBound("time", 12, 16),
			},
			expectedSlice: []float32{
				118, 119,
				122, 123,
			},
			expectedShape: []int{2, 2},
			expectedXAxis: newAxis("Sample", 12, 16, 2, 4),
			expectedYAxis: newAxis("Crossline", 10, 11, 2, 1),
			expectedGeo:   [][]float64{{14, 8}, {12, 11}},
		},
		{
			name:      "Out-Of-Bounds bounds errors",
			direction: "inline",
			lineno:    5,
			bounds: []Bound{
				newBound("time", 8, 20),
			},
			expectedSlice: []float32{},
			expectedErr:   NewInvalidArgument(""),
		},
		{
			name:      "Incorrect vertical domain",
			direction: "inline",
			lineno:    5,
			bounds: []Bound{
				newBound("depth", 8, 20),
			},
			expectedSlice: []float32{},
			expectedErr:   NewInvalidArgument(""),
		},
	}

	for _, testCase := range testCases {
		direction, err := GetAxis(testCase.direction)
		require.NoError(t, err)

		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetSlice(
			testCase.lineno,
			direction,
			testCase.bounds,
		)

		require.IsTypef(t, testCase.expectedErr, err,
			"[case: %v] Got: %v",
			testCase.name,
			err,
		)
		if testCase.expectedErr != nil {
			continue
		}

		slice, err := toFloat32(buf)
		require.NoErrorf(t, err, "[case: %v] Err: %v", testCase.name, err)

		require.Equalf(t, testCase.expectedSlice, *slice, "[case: %v]", testCase.name)

		buf, err = handle.GetSliceMetadata(
			testCase.lineno,
			direction,
			testCase.bounds,
		)
		require.NoError(t, err,
			"[case: %v] Failed to get slice metadata, err: %v",
			testCase.name,
			err,
		)

		var meta SliceMetadata
		err = json.Unmarshal(buf, &meta)
		require.NoErrorf(t, err,
			"[case: %v] Failed to unmarshall response, err: %v",
			testCase.name,
			err,
		)

		require.Equalf(t, testCase.expectedShape, meta.Shape,
			"[case: %v] Shape not equal",
			testCase.name,
		)

		require.Equalf(t, testCase.expectedXAxis, meta.X,
			"[case: %v] X axis not equal",
			testCase.name,
		)

		require.Equalf(t, testCase.expectedYAxis, meta.Y,
			"[case: %v] Y axis not equal",
			testCase.name,
		)

		require.Equalf(t, testCase.expectedGeo, meta.Geospatial,
			"[case: %v] Geospatial not equal",
			testCase.name,
		)
	}
}

func TestDepthAxis(t *testing.T) {
	testcases := []struct {
		name      string
		direction int
		err       error
	}{
		{
			name:      "Depth",
			direction: AxisDepth,
			err: NewInvalidArgument(
				"Cannot fetch depth slice for VDS file with vertical axis unit: ms",
			),
		},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		_, err := handle.GetSlice(0, testcase.direction, []Bound{})

		require.Equal(t, err, testcase.err)
	}
}

func TestSliceMetadata(t *testing.T) {
	lineno := 1
	direction := AxisJ
	expected := SliceMetadata{
		Array: Array{
			Format: "<f4",
		},
		X:          Axis{Annotation: "Sample", Min: 4, Max: 16, Samples: 4, StepSize: 4, Unit: "ms"},
		Y:          Axis{Annotation: "Inline", Min: 1, Max: 5, Samples: 3, StepSize: 2, Unit: "unitless"},
		Geospatial: [][]float64{{0, 3}, {12, 11}},
		Shape:      []int{3, 4},
	}
	handle, _ := NewVDSHandle(well_known)
	defer handle.Close()
	buf, err := handle.GetSliceMetadata(lineno, direction, []Bound{})
	require.NoErrorf(t, err, "Failed to retrieve slice metadata, err %v", err)

	var meta SliceMetadata

	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	err = dec.Decode(&meta)
	require.NoErrorf(t, err, "Failed to unmarshall response, err: %v", err)

	require.Equal(t, expected, meta)
}

func TestSliceMetadataAxisOrdering(t *testing.T) {
	testcases := []struct {
		name         string
		direction    int
		lineno       int
		expectedAxis []string
	}{
		{
			name:         "Inline",
			direction:    AxisInline,
			lineno:       3,
			expectedAxis: []string{"Sample", "Crossline"},
		},
		{
			name:         "I",
			lineno:       0,
			direction:    AxisI,
			expectedAxis: []string{"Sample", "Crossline"},
		},
		{
			name:         "Crossline",
			direction:    AxisCrossline,
			lineno:       10,
			expectedAxis: []string{"Sample", "Inline"},
		},
		{
			name:         "J",
			lineno:       0,
			direction:    AxisJ,
			expectedAxis: []string{"Sample", "Inline"},
		},
		{
			name:         "Time",
			direction:    AxisTime,
			lineno:       4,
			expectedAxis: []string{"Crossline", "Inline"},
		},
		{
			name:         "K",
			lineno:       0,
			direction:    AxisK,
			expectedAxis: []string{"Crossline", "Inline"},
		},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetSliceMetadata(
			testcase.lineno,
			testcase.direction,
			[]Bound{},
		)
		require.NoErrorf(t, err,
			"[case: %v] Failed to get slice metadata, err: %v",
			testcase.name,
			err,
		)

		var meta SliceMetadata
		err = json.Unmarshal(buf, &meta)
		require.NoErrorf(t, err,
			"[case: %v] Failed to unmarshall response, err: %v",
			testcase.name,
			err,
		)

		axis := []string{meta.X.Annotation, meta.Y.Annotation}

		require.Equalf(t, testcase.expectedAxis, axis, "[case: %v]", testcase.name)
	}
}

func TestSliceGeospatial(t *testing.T) {
	testcases := []struct {
		name      string
		direction int
		lineno    int
		expected  [][]float64
	}{
		{
			name:      "Inline",
			direction: AxisInline,
			lineno:    3,
			expected:  [][]float64{{8.0, 4.0}, {6.0, 7.0}},
		},
		{
			name:      "I",
			direction: AxisI,
			lineno:    1,
			expected:  [][]float64{{8.0, 4.0}, {6.0, 7.0}},
		},
		{
			name:      "Crossline",
			direction: AxisCrossline,
			lineno:    10,
			expected:  [][]float64{{2.0, 0.0}, {14.0, 8.0}},
		},
		{
			name:      "J",
			direction: AxisJ,
			lineno:    0,
			expected:  [][]float64{{2.0, 0.0}, {14.0, 8.0}},
		},
		{
			name:      "Time",
			direction: AxisTime,
			lineno:    4,
			expected:  [][]float64{{2.0, 0.0}, {14.0, 8.0}, {12.0, 11.0}, {0.0, 3.0}},
		},
		{
			name:      "K",
			direction: AxisK,
			lineno:    3,
			expected:  [][]float64{{2.0, 0.0}, {14.0, 8.0}, {12.0, 11.0}, {0.0, 3.0}},
		},
	}

	for _, testcase := range testcases {
		handle, _ := NewVDSHandle(well_known)
		defer handle.Close()
		buf, err := handle.GetSliceMetadata(
			testcase.lineno,
			testcase.direction,
			[]Bound{},
		)
		require.NoError(t, err,
			"[case: %v] Failed to get slice metadata, err: %v",
			testcase.name,
			err,
		)

		var meta SliceMetadata
		err = json.Unmarshal(buf, &meta)
		require.NoError(t, err,
			"[case: %v] Failed to unmarshall response, err: %v",
			testcase.name,
			err,
		)

		require.Equalf(
			t,
			testcase.expected,
			meta.Geospatial,
			"[case %v] Incorrect geospatial information",
			testcase.name,
		)
	}
}

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
		handle, _ := NewVDSHandle(well_known)
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
		handle, _ := NewVDSHandle(well_known)
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
		handle, _ := NewVDSHandle(well_known)
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
		handle, _ := NewVDSHandle(well_known)
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
	handle, _ := NewVDSHandle(well_known)
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
// at known datapoints. Openvds claims that data would still be the same for all
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

	handle, _ := NewVDSHandle(samples10)
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

// Also, as we can't check interpolation properly, check that beyond datapoints
// different values are returned by each algorithm
func TestFenceInterpolationDifferentBeyondDatapoints(t *testing.T) {
	fence := [][]float32{{3.2, 3}, {3.2, 6.3}, {1, 3}, {3.2, 3}, {5.4, 3}}
	var fillValue float32 = -999.25
	interpolationMethods := []string{"nearest", "linear", "cubic", "triangular", "angular"}
	for i, v1 := range interpolationMethods {
		interpolationMethod, _ := GetInterpolationMethod(v1)
		handle, _ := NewVDSHandle(well_known)
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

	handle, _ := NewVDSHandle(well_known)
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

func TestOnly3DSupported(t *testing.T) {
	handle, err := NewVDSHandle(prestack)
	if err != nil {
		handle.Close()
	}

	require.ErrorContains(t, err, "3 dimensions, got 4")
}

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

	handle, _ := NewVDSHandle(samples10)
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
		inbounds bool
	}{
		// 2 samples is the margin needed for interpolation
		{
			name:     "Top of window is 2 samples from first depth recording",
			values:   [][]float32{{16.00}},
			inbounds: true,
		},
		{
			name:     "Top of window is less than 2 samples from the top",
			values:   [][]float32{{13.00}},
			inbounds: false,
		},
		{
			name:     "Bottom of window is 2 samples from last depth recording",
			values:   [][]float32{{28.00}},
			inbounds: true,
		},
		{
			name:     "Bottom of window is less than 2 samples from last depth recording",
			values:   [][]float32{{31.00}},
			inbounds: false,
		},
		{
			name:     "Some values inbounds, some out of bounds",
			values:   [][]float32{{22.00, 32.00, 12.00}, {18.00, 31.00, 28.00}, {16.00, 15.00, 13.00}},
			inbounds: false,
		},
		{
			name:     "Fillvalue should not be bounds checked",
			values:   [][]float32{{-999.25}},
			inbounds: true,
		},
	}

	targetAttributes := []string{"min", "samplevalue"}
	const above = float32(4.0)
	const below = float32(4.0)
	const stepsize = float32(4.0)

	for _, testcase := range testcases {
		surface := samples10Surface(testcase.values)
		interpolationMethod, _ := GetInterpolationMethod("nearest")
		handle, _ := NewVDSHandle(samples10)
		defer handle.Close()
		_, boundsErr := handle.GetAttributesAlongSurface(
			surface,
			above,
			below,
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
				"[%s] Expected horizon value %f to throw out of bound",
				testcase.name,
				testcase.values[0][0],
			)
		}
	}
}

/**
 * The goal of this test is to test that the boundschecking in the horizontal
 * plane is correct - i.e. that we correctly populate the output array with
 * fillvalues. To acheive this we create horizons that are based on the VDS's
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
		handle, _ := NewVDSHandle(samples10)
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
		"max",
		"maxabs",
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
		{1.5, 2.5, -4.5, 10.5, fillValue, -8.5, fillValue, fillValue},                         // max
		{2.5, 2.5, 12.5, 10.5, fillValue, 24.5, fillValue, fillValue},                         // maxabs
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

	handle, _ := NewVDSHandle(samples10)
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

func TestAttributeBetweenSurfaces(t *testing.T) {
	expected := map[string][]float32{
		"samplevalue": {},
		"min":         {-1.5, -0.5, -8.5, 5.5, fillValue, -24.5, fillValue, fillValue},
		"max":         {2.5, 0.5, -8.5, 5.5, fillValue, -8.5, fillValue, fillValue},
		"maxabs":      {2.5, 0.5, 8.5, 5.5, fillValue, 24.5, fillValue, fillValue},
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

	handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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
		handle, _ := NewVDSHandle(samples10)
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

		handle, _ := NewVDSHandle(samples10)
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
		handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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
		handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(samples10)
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

	handle, _ := NewVDSHandle(well_known)
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
