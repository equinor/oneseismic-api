package vds

/*
#cgo LDFLAGS: -lopenvds
#cgo CXXFLAGS: -std=c++17
#include <vds.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

const (
	AxisI         = C.I
	AxisJ         = C.J
	AxisK         = C.K
	AxisInline    = C.INLINE
	AxisCrossline = C.CROSSLINE
	AxisDepth     = C.DEPTH
	AxisTime      = C.TIME
	AxisSample    = C.SAMPLE
)

const (
	CoordinateSystemIndex      = C.INDEX
	CoordinateSystemAnnotation = C.ANNOTATION
	CoordinateSystemCdp        = C.CDP
)

// @Description Axis description
type Axis struct {
	// Name/Annotation of axis
	Annotation string `json:"annotation" example:"Sample"`

	// Minimum axis value
	Min float64 `json:"min" example:"4.0"`

	// Maximum axis value
	Max float64 `json:"max" example:"4500.0"`

	// Number of samples along the axis
	Samples int `json:"samples" example:"1600"`

	// Axis units
	Unit string `json:"unit" example:"ms"`
} // @name Axis


// @Description The bounding box of the survey, defined by its 4 corner
// @Description coordinates. The bounding box is given in 3 different
// @Description coordinate systems. The points are sorted in the same order for
// @Description each system. E.g. [[1,2], [2,2], [2,3], [1,3]]
type BoundingBox struct {
	Cdp  [][]float64 `json:"cdp"`
	Ilxl [][]float64 `json:"ilxl"`
	Ij   [][]float64 `json:"ij"`
} //@name BoundingBox


// @Description Slice metadata
type SliceMetadata struct {
	// Data format is represented by a numpy-style formatcodes. E.g. f4 is 4
	// byte float, u1 is 1 byte unsinged int and u2 is 2 byte usigned int.
	// Currently format is always 4-byte floats, little endian (<f4).
	Format string `json:"format" example:"<f4"`

	// X-axis information
	X Axis `json:"x"`

	// Y-axis information
	Y Axis `json:"y"`
} // @name SliceMetadata

// @Description Metadata
type Metadata struct {
	// Coordinate reference system
	Crs string `json:"crs" example:"PROJCS[\"ED50 / UTM zone 31N\",..."`

	// The original input file name
	InputFileName string `json:"inputFileName" example:"file.segy"`

	// Bounding box
	BoundingBox BoundingBox `json:"boundingBox"`

	// Axis descriptions
	//
	// Describes the axes of the requested 2-dimensional slice.
	Axis []*Axis `json:"axis"`
} // @name Metadata

// @Description Fence metadata
type FenceMetadata struct {
	// Data format is represented by numpy-style formatcodes. For fence the
	// format is always 4-byte floats, little endian (<f4).
	Format string `json:"format" example:"<f4"`

	// Shape of the returned data fence.
	Shape []int `json:"shape" swaggertype:"array,integer" example:"10,50"`
} // @name FenceMetadata

func GetAxis(direction string) (int, error) {
	switch direction {
	case "i":         return AxisI,         nil
	case "j":         return AxisJ,         nil
	case "k":         return AxisK,         nil
	case "inline":    return AxisInline,    nil
	case "crossline": return AxisCrossline, nil
	case "depth":     return AxisDepth,     nil
	case "time":      return AxisTime,      nil
	case "sample":    return AxisSample,    nil
	default:
		options := "i, j, k, inline, crossline or depth/time/sample"
		msg := "invalid direction '%s', valid options are: %s"
		return -1, fmt.Errorf(msg, direction, options)
	}
}

func GetCoordinateSystem(coordinateSystem string) (int, error) {
	switch strings.ToLower(coordinateSystem) {
	case "ij":
		return CoordinateSystemIndex, nil
	case "ilxl":
		return CoordinateSystemAnnotation, nil
	case "cdp":
		return CoordinateSystemCdp, nil
	default:
		options := "ij, ilxl, cdp"
		msg := "coordinate system not recognized: '%s', valid options are: %s"
		return -1, fmt.Errorf(msg, coordinateSystem, options)
	}
}

func GetInterpolationMethod(interpolation string) (int, error) {
	switch strings.ToLower(interpolation) {
	case "":
		fallthrough
	case "nearest":
		return C.NEAREST, nil
	case "linear":
		return C.LINEAR, nil
	case "cubic":
		return C.CUBIC, nil
	case "angular":
		return C.ANGULAR, nil
	case "triangular":
		return C.TRIANGULAR, nil
	default:
		options := "nearest, linear, cubic, angular or triangular"
		msg := "invalid interpolation method '%s', valid options are: %s"
		return -1, fmt.Errorf(msg, interpolation, options)
	}
}

func GetAttributeType(attribute string) (int, error) {
	switch strings.ToLower(attribute) {
	case "samplevalue": return C.VALUE, nil
	case "min":         return C.MIN,   nil
	case "max":         return C.MAX,   nil
	case "mean":        return C.MEAN,  nil
	case "rms":         return C.RMS,   nil
	case "":            fallthrough
	default:
		options := "samplevalue, min, max, mean, rms"
		msg := "invalid attribute '%s', valid options are: %s"
		return -1, fmt.Errorf(msg, attribute, options)
	}
}

func GetMetadata(conn Connection) ([]byte, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	result := C.metadata(curl, ccred)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func GetSlice(conn Connection, lineno, direction int) ([]byte, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	result := C.slice(
		curl,
		ccred,
		C.int(lineno),
		C.enum_axis_name(direction),
	)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func GetSliceMetadata(conn Connection, direction int) ([]byte, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	result := C.slice_metadata(
		curl,
		ccred,
		C.enum_axis_name(direction),
	)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func GetFence(
	conn Connection,
	coordinateSystem int,
	coordinates [][]float32,
	interpolation int,
) ([]byte, error) {
	cvds := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(cvds))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	coordinate_len := 2
	ccoordinates := make([]C.float, len(coordinates) * coordinate_len)
	for i := range coordinates {

		if len(coordinates[i]) != coordinate_len  {
			msg := fmt.Sprintf(
				"invalid coordinate %v at position %d, expected [x y] pair",
				coordinates[i],
				i,
			)
			return nil, NewInvalidArgument(msg)
		}

		for j := range coordinates[i] {
			ccoordinates[i * coordinate_len  + j] = C.float(coordinates[i][j])
		}
	}

	result := C.fence(
		cvds,
		ccred,
		C.enum_coordinate_system(coordinateSystem),
		&ccoordinates[0],
		C.size_t(len(coordinates)),
		C.enum_interpolation_method(interpolation),
	)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func GetFenceMetadata(conn Connection, coordinates [][]float32) ([]byte, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	result := C.fence_metadata(
		curl,
		ccred,
		C.size_t(len(coordinates)),
	)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func getHorizon(
	conn          Connection,
	data          [][]float32,
	originX       float32,
	originY       float32,
	increaseX     float32,
	increaseY     float32,
	rotation      float32,
	fillValue     float32,
	above         float32,
	below         float32,
	interpolation int,
) (*C.struct_response, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	nrows := len(data)
	ncols := len(data[0])

	cdata := make([]C.float, nrows * ncols)
	for i := range data {
		if len(data[i]) != ncols  {
			msg := fmt.Sprintf(
				"Surface rows are not of the same length. "+
					"Row 0 has %d elements. Row %d has %d elements",
				ncols, i, len(data[i]),
			)
			return nil, NewInvalidArgument(msg)
		}

		for j := range data[i] {
			cdata[i * ncols  + j] = C.float(data[i][j])
		}
	}

	result := C.horizon(
		curl,
		ccred,
		&cdata[0],
		C.size_t(nrows),
		C.size_t(ncols),
		C.float(originX),
		C.float(originY),
		C.float(increaseX),
		C.float(increaseY),
		C.float(rotation),
		C.float(fillValue),
		C.float(above),
		C.float(below),
		C.enum_interpolation_method(interpolation),
	)

	if result.err != nil {
		err := C.GoString(result.err)
		C.response_delete(&result)
		return nil, errors.New(err)
	}

	return &result, nil
}

func GetHorizonMetadata(conn Connection, data [][]float32) ([]byte, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	result := C.horizon_metadata(
		curl,
		ccred,
		C.size_t(len(data)),
		C.size_t(len(data[0])),
	)

	defer C.response_delete(&result)

	if result.err != nil {
		err := C.GoString(result.err)
		return nil, errors.New(err)
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func GetAttributes(
	conn          Connection,
	data          [][]float32,
	originX       float32,
	originY       float32,
	increaseX     float32,
	increaseY     float32,
	rotation      float32,
	fillValue     float32,
	above         float32,
	below         float32,
	attributes    []string,
	interpolation int,
) ([][]byte, error) {
	var targetAttributes []int
	for _, attr := range attributes {
		id, err := GetAttributeType(attr)
		if err != nil {
			return nil, err
		}
		targetAttributes = append(targetAttributes, id);
	}

	horizon, err := getHorizon(
		conn,
		data,
		originX,
		originY,
		increaseX,
		increaseY,
		rotation,
		fillValue,
		above,
		below,
		interpolation,
	)
	if err != nil {
		return nil, err
	}
	defer C.response_delete(horizon)

	var nrows   = len(data)
	var ncols   = len(data[0])
	var hsize   = nrows * ncols
	var mapsize = hsize * 4
	var vsize   = int(horizon.size) / mapsize

	cattributes := make([]C.enum_attribute, len(targetAttributes))
	for i := range targetAttributes {
		cattributes[i] = C.enum_attribute(targetAttributes[i])
	}

	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	buffer := C.attribute(
		curl,
		ccred,
		horizon.data,
		C.size_t(hsize),
		C.size_t(vsize),
		C.float(fillValue),
		&cattributes[0],
		C.size_t(len(targetAttributes)),
		C.float(above),
	)
	defer C.response_delete(&buffer)

	if buffer.err != nil {
		err := C.GoString(buffer.err)
		return nil, errors.New(err)
	}

	out := make([][]byte, len(targetAttributes))
	for i := range targetAttributes {
		out[i] = C.GoBytes(
			unsafe.Add(unsafe.Pointer(buffer.data), uintptr(i * mapsize)),
			C.int(mapsize),
		)
	}

	return out, nil
}
