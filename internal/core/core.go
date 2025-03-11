package core

/*
#cgo LDFLAGS: -lopenvds
#cgo CXXFLAGS: -std=c++17
#include <capi.h>
#include <ctypes.h>
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

const (
	BinaryOperatorInvalidOperator = C.INVALID_OPERATOR
	BinaryOperatorNoOperator      = C.NO_OPERATOR
	BinaryOperatorAddition        = C.ADDITION
	BinaryOperatorSubtraction     = C.SUBTRACTION
	BinaryOperatorMultiplication  = C.MULTIPLICATION
	BinaryOperatorDivision        = C.DIVISION
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

	// Distance from one sample to the next
	StepSize float64 `json:"stepsize" example:"4.0"`

	// Axis units
	Unit string `json:"unit" example:"ms"`
} // @name Axis

// @Description Geometrical plane with depth/time data points
type RegularSurface struct {
	// Values / height-map
	Values [][]float32 `json:"values" binding:"required"`

	// Rotation of the X-axis (East), counter-clockwise, in degrees
	Rotation *float32 `json:"rotation" binding:"required" example:"33.78"`

	// X-coordinate of the origin
	Xori *float32 `json:"xori" binding:"required" example:"-324.1"`

	// Y-coordinate of the origin
	Yori *float32 `json:"yori" binding:"required" example:"6721.33"`

	// X-increment - The physical distance between height-map columns
	Xinc float32 `json:"xinc" binding:"required" example:"8.12"`

	// Y-increment - The physical distance between height-map rows
	Yinc float32 `json:"yinc" binding:"required" example:"-1.02"`

	// Any sample in the input values with value == fillValue will be ignored
	// and the fillValue will be used in the amplitude map.
	// I.e. for any [i, j] where values[i][j] == fillValue then
	// output[i][j] == fillValue.
	// Additionally, the fillValue is used for any point of the surface that
	// falls outside the bounds of the seismic volume.
	FillValue *float32 `json:"fillValue" binding:"required" example:"-999.25"`
} // @name RegularSurface

// @Description The bounding box of the survey, defined by its 4 corner
// @Description coordinates. The bounding box is given in 3 different
// @Description coordinate systems. The points are sorted in the same order for
// @Description each system. E.g. [[1,2], [2,2], [2,3], [1,3]]
type BoundingBox struct {
	Cdp  [][]float64 `json:"cdp"`
	Ilxl [][]float64 `json:"ilxl"`
	Ij   [][]float64 `json:"ij"`
} //@name BoundingBox

type Array struct {
	// Data format is represented by numpy-style format codes. Currently the
	// format is always 4-byte floats, little endian (<f4).
	Format string `json:"format" example:"<f4"`

	// Shape of the returned data
	Shape []int `json:"shape" swaggertype:"array,integer" example:"10,50"`
}

// @Description Slice bounds.
type Bound struct {
	// Direction of the bound. See SliceRequest.Direction for valid options
	Direction *string `json:"direction" binding:"required" example:"inline"`

	// Lower bound - inclusive
	Lower *int `json:"lower" binding:"required" example:"100"`

	// Upper bound - inclusive
	// Upper bound must be greater or equal to lower bound
	Upper *int `json:"upper" binding:"required" example:"200"`
} // @name SliceBound

// @Description Slice metadata
type SliceMetadata struct {
	Array

	// X-axis information
	X Axis `json:"x"`

	// Y-axis information
	Y Axis `json:"y"`

	/* Override shape for docs */

	// Shape of the returned slice. Equals to [Y.Samples, X.Samples]
	Shape []int `json:"shape" swaggertype:"array,integer" example:"10,50"`

	// Horizontal bounding box of the slice. For inline/crossline slices this
	// is a linestring, while for time/depth slices this is a polygon. If the
	// slice is not cropped, the polygon is the bounding box of the cube.
	Geospatial [][]float64 `json:"geospatial"`
} // @name SliceMetadata

// @Description Metadata
type Metadata struct {
	// Coordinate reference system
	Crs string `json:"crs" example:"PROJCS[\"ED50 / UTM zone 31N\",..."`

	// The original input file name
	InputFileName string `json:"inputFileName" example:"file.segy"`

	// Import time stamp in ISO8601 format
	ImportTimeStamp string `json:"importTimeStamp" example:"2021-02-18T21:54:42.123Z"`

	// Bounding box
	BoundingBox BoundingBox `json:"boundingBox"`

	// Axis descriptions
	//
	// Describes the axes of the requested 2-dimensional slice.
	Axis []*Axis `json:"axis"`
} // @name Metadata

// @Description Fence metadata
type FenceMetadata struct {
	Array
} // @name FenceMetadata

// @Description Attribute metadata
type AttributeMetadata struct {
	Array
} // @name AttributeMetadata

func GetAxis(direction string) (int, error) {
	switch direction {
	case "i":
		return AxisI, nil
	case "j":
		return AxisJ, nil
	case "k":
		return AxisK, nil
	case "inline":
		return AxisInline, nil
	case "crossline":
		return AxisCrossline, nil
	case "depth":
		return AxisDepth, nil
	case "time":
		return AxisTime, nil
	case "sample":
		return AxisSample, nil
	default:
		options := "i, j, k, inline, crossline or depth/time/sample"
		msg := "invalid direction '%s', valid options are: %s"
		return -1, NewInvalidArgument(fmt.Sprintf(msg, direction, options))
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
		return -1, NewInvalidArgument(fmt.Sprintf(msg, coordinateSystem, options))
	}
}

func GetBinaryOperator(binaryOperator string) (uint32, error) {
	switch strings.ToLower(binaryOperator) {
	case "":
		return BinaryOperatorNoOperator, nil
	case "addition":
		return BinaryOperatorAddition, nil
	case "subtraction":
		return BinaryOperatorSubtraction, nil
	case "multiplication":
		return BinaryOperatorMultiplication, nil
	case "division":
		return BinaryOperatorDivision, nil
	default:
		options := "addition, subtraction, multiplication, division"
		msg := "Binary operator not recognized: '%s', valid options are: %s"
		return BinaryOperatorNoOperator, NewInvalidArgument(fmt.Sprintf(msg, binaryOperator, options))
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
		return -1, NewInvalidArgument(fmt.Sprintf(msg, interpolation, options))
	}
}

func GetAttributeType(attribute string) (int, error) {
	switch strings.ToLower(attribute) {
	case "samplevalue":
		return C.VALUE, nil
	case "min":
		return C.MIN, nil
	case "min_at":
		return C.MINAT, nil
	case "max":
		return C.MAX, nil
	case "max_at":
		return C.MAXAT, nil
	case "maxabs":
		return C.MAXABS, nil
	case "maxabs_at":
		return C.MAXABSAT, nil
	case "mean":
		return C.MEAN, nil
	case "meanabs":
		return C.MEANABS, nil
	case "meanpos":
		return C.MEANPOS, nil
	case "meanneg":
		return C.MEANNEG, nil
	case "median":
		return C.MEDIAN, nil
	case "rms":
		return C.RMS, nil
	case "var":
		return C.VAR, nil
	case "sd":
		return C.SD, nil
	case "sumpos":
		return C.SUMPOS, nil
	case "sumneg":
		return C.SUMNEG, nil
	case "":
		fallthrough
	default:
		options := []string{
			"samplevalue", "min", "min_at", "max", "max_at", "maxabs", "maxabs_at",
			"mean", "meanabs", "meanpos", "meanneg", "median", "rms", "var", "sd",
			"sumpos", "sumneg",
		}
		msg := "invalid attribute '%s', valid options are: %s"
		return -1, NewInvalidArgument(fmt.Sprintf(
			msg,
			attribute,
			strings.Join(options, ", "),
		))
	}
}

/** Translate C status codes into Go error types */
func toError(status C.int, ctx *C.Context) error {
	if status == C.STATUS_OK {
		return nil
	}

	msg := C.GoString(C.errmsg(ctx))

	switch status {
	case C.STATUS_NULLPTR_ERROR:
		fallthrough
	case C.STATUS_RUNTIME_ERROR:
		return NewInternalError(msg)
	case C.STATUS_BAD_REQUEST:
		return NewInvalidArgument(msg)
	default:
		return errors.New(msg)
	}
}

type cRegularSurface struct {
	cSurface *C.struct_RegularSurface
	cData    []C.float
}

func (r *cRegularSurface) get() *C.struct_RegularSurface {
	return r.cSurface
}

func (r *cRegularSurface) Close() error {
	var cCtx = C.context_new()
	defer C.context_free(cCtx)

	cErr := C.regular_surface_free(cCtx, r.cSurface)
	if err := toError(cErr, cCtx); err != nil {
		return err
	}

	return nil
}

func (surface *RegularSurface) toCdata(shift float32) ([]C.float, error) {
	nrows := len(surface.Values)
	ncols := len(surface.Values[0])

	cdata := make([]C.float, nrows*ncols)

	for i, row := range surface.Values {
		if len(row) != ncols {
			msg := fmt.Sprintf(
				"Surface rows are not of the same length. "+
					"Row 0 has %d elements. Row %d has %d elements",
				ncols, i, len(row),
			)
			return nil, NewInvalidArgument(msg)
		}

		for j, value := range row {
			if value == *surface.FillValue {
				cdata[i*ncols+j] = C.float(value)
			} else {
				cdata[i*ncols+j] = C.float(value + shift)
			}
		}
	}
	return cdata, nil
}

func (s *RegularSurface) ToString() string {
	return fmt.Sprintf("{(ncols: %d, nrows: %d), Rotation: %.2f, "+
		"Origin: [%.2f, %.2f], Increment: [%.2f, %.2f], FillValue: %.2f}, ",
		len(s.Values[0]),
		len(s.Values),
		*s.Rotation,
		*s.Xori,
		*s.Yori,
		s.Xinc,
		s.Yinc,
		*s.FillValue)
}

func (surface *RegularSurface) toCRegularSurface(cdata []C.float) (cRegularSurface, error) {
	nrows := len(surface.Values)
	ncols := len(surface.Values[0])

	if len(cdata) == 0 {
		msg := "Surface should contain at least one value"
		return cRegularSurface{}, NewInvalidArgument(msg)
	}

	var cCtx = C.context_new()
	defer C.context_free(cCtx)

	var cSurface *C.struct_RegularSurface
	cErr := C.regular_surface_new(
		cCtx,
		&cdata[0],
		C.size_t(nrows),
		C.size_t(ncols),
		C.float(*surface.Xori),
		C.float(*surface.Yori),
		C.float(surface.Xinc),
		C.float(surface.Yinc),
		C.float(*surface.Rotation),
		C.float(*surface.FillValue),
		&cSurface,
	)

	if err := toError(cErr, cCtx); err != nil {
		C.regular_surface_free(cCtx, cSurface)
		return cRegularSurface{}, err
	}

	return cRegularSurface{cSurface: cSurface, cData: cdata}, nil
}

type DSHandle struct {
	dataHandle *C.struct_DataHandle
	ctx        *C.struct_Context
}

func (v DSHandle) DataHandle() *C.struct_DataHandle {
	return v.dataHandle
}

func (v DSHandle) context() *C.struct_Context {
	return v.ctx
}

func (v DSHandle) Error(status C.int) error {
	return toError(status, v.context())
}

func (v DSHandle) Close() error {
	defer C.context_free(v.ctx)

	cerr := C.datahandle_free(v.ctx, v.dataHandle)
	return toError(cerr, v.ctx)
}

func NewDSHandle(connection Connection) (DSHandle, error) {
	return CreateDSHandle([]Connection{connection}, BinaryOperatorNoOperator)
}

func contains(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func CreateDSHandle(connections []Connection, operator uint32) (DSHandle, error) {

	if len(connections) == 0 || len(connections) > 2 {
		return DSHandle{}, NewInvalidArgument("Invalid number of connections provided")
	}

	var cctx = C.context_new()
	var dataHandle *C.struct_DataHandle
	var cerr C.int

	curlA := C.CString(connections[0].Url())
	defer C.free(unsafe.Pointer(curlA))

	ccredA := C.CString(connections[0].ConnectionString())
	defer C.free(unsafe.Pointer(ccredA))

	if len(connections) == 1 {
		cerr = C.single_datahandle_new(cctx, curlA, ccredA, &dataHandle)
	} else if len(connections) == 2 {
		curlB := C.CString(connections[1].Url())
		defer C.free(unsafe.Pointer(curlB))

		ccredB := C.CString(connections[1].ConnectionString())
		defer C.free(unsafe.Pointer(ccredB))

		cerr = C.double_datahandle_new(cctx, curlA, ccredA, curlB, ccredB, operator, &dataHandle)
	}

	if err := toError(cerr, cctx); err != nil {
		defer C.context_free(cctx)
		return DSHandle{}, err
	}

	return DSHandle{dataHandle: dataHandle, ctx: cctx}, nil
}

func (v DSHandle) GetMetadata() ([]byte, error) {
	var result C.struct_response = C.response_create()
	cerr := C.metadata(v.context(), v.DataHandle(), &result)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}
