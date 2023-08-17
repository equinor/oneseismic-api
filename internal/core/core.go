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
	"math"
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

// @Description Geometrical plane with depth/time datapoints
type RegularSurface struct {
	// Values / height-map
	Values [][]float32 `json:"values" binding:"required"`

	// Rotation of the X-axis (East), counter-clockwise, in degrees
	Rotation *float32 `json:"rotation" binding:"required"`

	// X-coordinate of the origin
	Xori *float32 `json:"xori" binding:"required"`

	// Y-coordinate of the origin
	Yori *float32 `json:"yori" binding:"required"`

	// X-increment - The physical distance between height-map columns
	Xinc float32 `json:"xinc" binding:"required"`

	// Y-increment - The physical distance between height-map rows
	Yinc float32 `json:"yinc" binding:"required"`

	// Any sample in the input values with value == fillValue will be ignored
	// and the fillValue will be used in the amplitude map.
	// I.e. for any [i, j] where values[i][j] == fillValue then
	// output[i][j] == fillValue.
	// Additionally, the fillValue is used for any point of the surface that
	// falls outside the bounds of the seismic volume.
	FillValue *float32 `json:"fillValue" binding:"required"`
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
	// Data format is represented by numpy-style formatcodes. Currently the
	// format is always 4-byte floats, little endian (<f4).
	Format string `json:"format" example:"<f4"`

	// Shape of the returned data
	Shape []int `json:"shape" swaggertype:"array,integer" example:"10,50"`
}

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
	// is a linestring, while for time/depth slices this is essentially the
	// bounding box of the volume.
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
	case "max":
		return C.MAX, nil
	case "maxabs":
		return C.MAXABS, nil
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
			"samplevalue", "min", "max", "maxabs", "mean", "meanabs", "meanpos",
			"meanneg", "median", "rms", "var", "sd", "sumpos", "sumneg",
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

func (surface *RegularSurface) toCdata(shift float32) ([]C.float, error){
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

func (surface *RegularSurface) toCRegularSurface(cdata []C.float) (cRegularSurface, error) {
	nrows := len(surface.Values)
	ncols := len(surface.Values[0])

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

type VDSHandle struct {
	handle *C.struct_DataHandle
	ctx    *C.struct_Context
}

func (v VDSHandle) Handle() *C.struct_DataHandle {
	return v.handle
}

func (v VDSHandle) context() *C.struct_Context {
	return v.ctx
}

func (v VDSHandle) Error(status C.int) error {
	return toError(status, v.context())
}

func (v VDSHandle) Close() error {
	defer C.context_free(v.ctx)

	cerr := C.datahandle_free(v.ctx, v.handle)
	return toError(cerr, v.ctx)
}

func NewVDSHandle(conn Connection) (VDSHandle, error) {
	curl := C.CString(conn.Url())
	defer C.free(unsafe.Pointer(curl))

	ccred := C.CString(conn.ConnectionString())
	defer C.free(unsafe.Pointer(ccred))

	var cctx = C.context_new()
	var handle *C.struct_DataHandle

	cerr := C.datahandle_new(cctx, curl, ccred, &handle)

	if err := toError(cerr, cctx); err != nil {
		defer C.context_free(cctx)
		return VDSHandle{}, err
	}

	return VDSHandle{handle: handle, ctx: cctx}, nil
}

func (v VDSHandle) GetMetadata() ([]byte, error) {
	var result C.struct_response
	cerr := C.metadata(v.context(), v.Handle(), &result)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func (v VDSHandle) GetSlice(lineno, direction int) ([]byte, error) {
	var result C.struct_response
	cerr := C.slice(
		v.context(),
		v.Handle(),
		C.int(lineno),
		C.enum_axis_name(direction),
		&result,
	)

	defer C.response_delete(&result)
	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func (v VDSHandle) GetSliceMetadata(lineno, direction int) ([]byte, error) {
	var result C.struct_response
	cerr := C.slice_metadata(
		v.context(),
		v.Handle(),
		C.int(lineno),
		C.enum_axis_name(direction),
		&result,
	)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func (v VDSHandle) GetFence(
	coordinateSystem int,
	coordinates [][]float32,
	interpolation int,
	fillValue *float32,
) ([]byte, error) {
	coordinate_len := 2
	ccoordinates := make([]C.float, len(coordinates)*coordinate_len)
	for i := range coordinates {

		if len(coordinates[i]) != coordinate_len {
			msg := fmt.Sprintf(
				"invalid coordinate %v at position %d, expected [x y] pair",
				coordinates[i],
				i,
			)
			return nil, NewInvalidArgument(msg)
		}

		for j := range coordinates[i] {
			ccoordinates[i*coordinate_len+j] = C.float(coordinates[i][j])
		}
	}

	var result C.struct_response
	cerr := C.fence(
		v.context(),
		v.Handle(),
		C.enum_coordinate_system(coordinateSystem),
		&ccoordinates[0],
		C.size_t(len(coordinates)),
		C.enum_interpolation_method(interpolation),
		(*C.float)(fillValue),
		&result,
	)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func (v VDSHandle) GetFenceMetadata(coordinates [][]float32) ([]byte, error) {
	var result C.struct_response
	cerr := C.fence_metadata(
		v.context(),
		v.Handle(),
		C.size_t(len(coordinates)),
		&result,
	)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (v VDSHandle) GetAttributeMetadata(data [][]float32) ([]byte, error) {
	var result C.struct_response
	cerr := C.attribute_metadata(
		v.context(),
		v.Handle(),
		C.size_t(len(data)),
		C.size_t(len(data[0])),
		&result,
	)

	defer C.response_delete(&result)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	buf := C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))
	return buf, nil
}

func (v VDSHandle) GetAttributes(
	primarySurface RegularSurface,
	above float32,
	below float32,
	stepsize float32,
	attributes []string,
	interpolation int,
) ([][]byte, error) {
	var targetAttributes []int
	for _, attr := range attributes {
		id, err := GetAttributeType(attr)
		if err != nil {
			return nil, err
		}
		targetAttributes = append(targetAttributes, id)
	}

	if above < 0 || below < 0 {
		msg := fmt.Sprintf(
			"Above and below must be positive. "+
				"Above was %f, below was %f",
			above, below,
		)
		return nil, NewInvalidArgument(msg)
	}

	var nrows = len(primarySurface.Values)
	var ncols = len(primarySurface.Values[0])
	var hsize = nrows * ncols

	cAttributes := make([]C.enum_attribute, len(targetAttributes))
	for i := range targetAttributes {
		cAttributes[i] = C.enum_attribute(targetAttributes[i])
	}

	cPrimarySurfaceData, err := primarySurface.toCdata(0)
	if err != nil {
		return nil, err
	}

	cPrimarySurface, err := primarySurface.toCRegularSurface(cPrimarySurfaceData)
	if err != nil {
		return nil, err
	}
	defer cPrimarySurface.Close()

	cTopSurfaceData, err := primarySurface.toCdata(-above)
	if err != nil {
		return nil, err
	}

	cTopSurface, err := primarySurface.toCRegularSurface(cTopSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cTopSurface.Close()

	cBottomSurfaceData, err := primarySurface.toCdata(below)
	if err != nil {
		return nil, err
	}
	cBottomSurface, err := primarySurface.toCRegularSurface(cBottomSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cBottomSurface.Close()

	dataOffsetSize := hsize + 1
	dataOffset := make([]C.size_t, dataOffsetSize)

	cerr := C.horizon_buffer_offsets(
		v.context(),
		v.Handle(),
		cPrimarySurface.get(),
		cTopSurface.get(),
		cBottomSurface.get(),
		&dataOffset[0],
		C.size_t(dataOffsetSize),
	)
	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	horizonBufferSize := dataOffset[hsize] * 4 //size of float
	if horizonBufferSize == 0 {
		//empty array can't be referenced
		horizonBufferSize = 1
	}

	horizon, err := v.fetchHorizon(
		cPrimarySurface,
		cTopSurface,
		cBottomSurface,
		nrows,
		ncols,
		dataOffset,
		interpolation,
		C.size_t(horizonBufferSize),
	)
	if err != nil {
		return nil, err
	}

	return v.calculateAttributes(
		cPrimarySurface,
		cTopSurface,
		cBottomSurface,
		hsize,
		dataOffset,
		horizon,
		C.size_t(horizonBufferSize),
		cAttributes,
		stepsize,
	)
}

func (v VDSHandle) fetchHorizon(
	cPrimarySurface cRegularSurface,
	cTopSurface cRegularSurface,
	cBottomSurface cRegularSurface,
	nrows int,
	ncols int,
	dataOffset []C.size_t,
	interpolation int,
	horizonBufferSize C.size_t,
) ([]byte, error) {
	hsize := nrows * ncols
	buffer := make([]byte, horizonBufferSize)

	// note that it is possible to hit go's own goroutines limit
	// but we do not deal with it here

	// max number of goroutines running at the same time
	// too low number doesn't utilize all CPU, too high overuses it
	// value should be experimented with
	maxConcurrentGoroutines := max(nrows/2, 1)
	guard := make(chan struct{}, maxConcurrentGoroutines)

	// the size of the data processed in one goroutine
	// decides how many parts data is split into
	// value should be experimented with
	chunkSize := max(nrows, 1)

	from := 0
	to := from + chunkSize

	errs := make(chan error, hsize/chunkSize+1)
	nRoutines := 0

	for from < hsize {
		guard <- struct{}{} // block if guard channel is filled
		go func(from, to int) {
			var cCtx = C.context_new()
			defer C.context_free(cCtx)

			cerr := C.horizon(
				cCtx,
				v.Handle(),
				cPrimarySurface.get(),
				cTopSurface.get(),
				cBottomSurface.get(),
				&dataOffset[0],
				C.enum_interpolation_method(interpolation),
				C.size_t(from),
				C.size_t(to),
				unsafe.Pointer(&buffer[0]),
			)
			errs <- toError(cerr, cCtx)
			<-guard
		}(from, to)

		nRoutines += 1

		from += chunkSize
		to = min(to+chunkSize, hsize)
	}

	// Wait for all gorutines to finish and collect any errors
	var computeErrors []error
	for i := 0; i < nRoutines; i++ {
		err := <-errs
		if err != nil {
			computeErrors = append(computeErrors, err)
		}
	}

	if len(computeErrors) > 0 {
		return nil, computeErrors[0]
	}
	return buffer, nil
}

func (v VDSHandle) calculateAttributes(
	cPrimarySurface cRegularSurface,
	cTopSurface cRegularSurface,
	cBottomSurface cRegularSurface,
	hsize int,
	dataOffset []C.size_t,
	horizon []byte,
	horizonSize C.size_t,
	cAttributes []uint32,
	stepsize float32,
) ([][]byte, error) {
	nAttributes := len(cAttributes)
	var mapsize = hsize * 4
	buffer := make([]byte, mapsize*nAttributes)

	maxConcurrency := 32
	windowsPerRoutine := int(math.Ceil(float64(hsize) / float64(maxConcurrency)))

	errs := make(chan error, maxConcurrency)

	from := 0
	remaining := hsize
	nRoutines := 0
	for remaining > 0 {
		nRoutines++

		size := min(windowsPerRoutine, remaining)
		to := from + size

		go func(from, to int) {
			var cCtx = C.context_new()
			defer C.context_free(cCtx)

			cErr := C.attribute(
				cCtx,
				v.Handle(),
				cPrimarySurface.get(),
				cTopSurface.get(),
				cBottomSurface.get(),
				&dataOffset[0],
				unsafe.Pointer(&horizon[0]),
				horizonSize,
				&cAttributes[0],
				C.size_t(nAttributes),
				C.float(stepsize),
				C.size_t(from),
				C.size_t(to),
				unsafe.Pointer(&buffer[0]),
			)

			errs <- toError(cErr, cCtx)
		}(from, to)

		from = to
		remaining -= size
	}

	// Wait for all gorutines to finish and collect any errors
	var computeErrors []error
	for i := 0; i < nRoutines; i++ {
		err := <-errs
		if err != nil {
			computeErrors = append(computeErrors, err)
		}
	}

	if len(computeErrors) > 0 {
		return nil, computeErrors[0]
	}

	out := make([][]byte, nAttributes)
	for i := 0; i < nAttributes; i++ {
		out[i] = buffer[i*mapsize : (i+1)*mapsize]
	}

	return out, nil
}
