package core

/*
#include <capi.h>
#include <ctypes.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func (v DSHandle) GetAttributeMetadata(data [][]float32) ([]byte, error) {
	var result C.struct_response = C.response_create()
	cerr := C.attribute_metadata(
		v.context(),
		v.DataHandle(),
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

func (v DSHandle) GetAttributesAlongSurface(
	referenceSurface RegularSurface,
	above float32,
	below float32,
	stepsize float32,
	attributes []string,
	interpolation int,
) ([][]byte, error) {
	targetAttributes, err := v.normalizeAttributes(attributes)
	if err != nil {
		return nil, err
	}

	if above < 0 || below < 0 {
		msg := fmt.Sprintf(
			"Above and below must be positive. "+
				"Above was %f, below was %f",
			above, below,
		)
		return nil, NewInvalidArgument(msg)
	}

	var nrows = len(referenceSurface.Values)
	var ncols = len(referenceSurface.Values[0])

	cReferenceSurfaceData, err := referenceSurface.toCdata(0)
	if err != nil {
		return nil, err
	}

	cReferenceSurface, err := referenceSurface.toCRegularSurface(cReferenceSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cReferenceSurface.Close()

	cTopSurfaceData, err := referenceSurface.toCdata(-above)
	if err != nil {
		return nil, err
	}

	cTopSurface, err := referenceSurface.toCRegularSurface(cTopSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cTopSurface.Close()

	cBottomSurfaceData, err := referenceSurface.toCdata(below)
	if err != nil {
		return nil, err
	}
	cBottomSurface, err := referenceSurface.toCRegularSurface(cBottomSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cBottomSurface.Close()

	return v.getAttributes(
		cReferenceSurface,
		cTopSurface,
		cBottomSurface,
		nrows,
		ncols,
		targetAttributes,
		interpolation,
		stepsize,
	)
}

func (v DSHandle) GetAttributesBetweenSurfaces(
	primarySurface RegularSurface,
	secondarySurface RegularSurface,
	stepsize float32,
	attributes []string,
	interpolation int,
) ([][]byte, error) {
	targetAttributes, err := v.normalizeAttributes(attributes)
	if err != nil {
		return nil, err
	}

	var nrows = len(primarySurface.Values)
	var ncols = len(primarySurface.Values[0])
	var hsize = nrows * ncols

	cPrimarySurfaceData, err := primarySurface.toCdata(0)
	if err != nil {
		return nil, err
	}
	cPrimarySurface, err := primarySurface.toCRegularSurface(cPrimarySurfaceData)
	if err != nil {
		return nil, err
	}
	defer cPrimarySurface.Close()

	cSecondarySurfaceData, err := secondarySurface.toCdata(0)
	if err != nil {
		return nil, err
	}
	cSecondarySurface, err := secondarySurface.toCRegularSurface(cSecondarySurfaceData)
	if err != nil {
		return nil, err
	}
	defer cSecondarySurface.Close()

	cAlignedSurfaceData := make([]C.float, hsize)
	cAlignedSurface, err := primarySurface.toCRegularSurface(cAlignedSurfaceData)
	if err != nil {
		return nil, err
	}
	defer cAlignedSurface.Close()

	var primaryIsTop C.int

	cerr := C.align_surfaces(
		v.context(),
		cPrimarySurface.get(),
		cSecondarySurface.get(),
		cAlignedSurface.get(),
		&primaryIsTop,
	)

	if err := v.Error(cerr); err != nil {
		return nil, err
	}

	var cTopSurface cRegularSurface
	var cBottomSurface cRegularSurface

	if primaryIsTop != 0 {
		cTopSurface = cPrimarySurface
		cBottomSurface = cAlignedSurface
	} else {
		cTopSurface = cAlignedSurface
		cBottomSurface = cPrimarySurface
	}

	return v.getAttributes(
		cPrimarySurface,
		cTopSurface,
		cBottomSurface,
		nrows,
		ncols,
		targetAttributes,
		interpolation,
		stepsize,
	)
}

func (v DSHandle) normalizeAttributes(
	attributes []string,
) ([]int, error) {
	var targetAttributes []int
	for _, attr := range attributes {
		id, err := GetAttributeType(attr)
		if err != nil {
			return nil, err
		}
		targetAttributes = append(targetAttributes, id)
	}
	return targetAttributes, nil
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

func (v DSHandle) getAttributes(
	cReferenceSurface cRegularSurface,
	cTopSurface cRegularSurface,
	cBottomSurface cRegularSurface,
	nrows int,
	ncols int,
	targetAttributes []int,
	interpolation int,
	stepsize float32,
) ([][]byte, error) {
	var hsize = nrows * ncols

	var cSubVolume *C.struct_SurfaceBoundedSubVolume
	var cCtx = C.context_new()
	defer C.context_free(cCtx)
	cerr := C.subvolume_new(
		cCtx,
		v.DataHandle(),
		cReferenceSurface.get(),
		cTopSurface.get(),
		cBottomSurface.get(),
		&cSubVolume,
	)

	if err := toError(cerr, cCtx); err != nil {
		return nil, err
	}
	defer C.subvolume_free(cCtx, cSubVolume)

	if len(targetAttributes) == 0 {
		msg := "Attributes should contain at least one value"
		return nil, NewInvalidArgument(msg)
	}

	cAttributes := make([]C.enum_attribute, len(targetAttributes))
	for i := range targetAttributes {
		cAttributes[i] = C.enum_attribute(targetAttributes[i])
	}

	nAttributes := len(cAttributes)
	var mapsize = hsize * 4
	buffer := make([]byte, mapsize*nAttributes)

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

			cerr_attributes := C.attribute(
				cCtx,
				v.DataHandle(),
				cSubVolume,
				C.enum_interpolation_method(interpolation),
				&cAttributes[0],
				C.size_t(nAttributes),
				C.float(stepsize),
				C.size_t(from),
				C.size_t(to),
				unsafe.Pointer(&buffer[0]),
			)

			errs <- toError(cerr_attributes, cCtx)
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

	out := make([][]byte, nAttributes)
	for i := 0; i < nAttributes; i++ {
		out[i] = buffer[i*mapsize : (i+1)*mapsize]
	}

	return out, nil
}
