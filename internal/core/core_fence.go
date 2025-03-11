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

func (v DSHandle) GetFence(
	coordinateSystem int,
	coordinates [][]float32,
	interpolation int,
	fillValue *float32,
) ([]byte, error) {

	if len(coordinates) == 0 {
		msg := "Coordinates should contain at least one value"
		return nil, NewInvalidArgument(msg)
	}

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

	var result C.struct_response = C.response_create()
	cerr := C.fence(
		v.context(),
		v.DataHandle(),
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

func (v DSHandle) GetFenceMetadata(coordinates [][]float32) ([]byte, error) {
	var result C.struct_response = C.response_create()
	cerr := C.fence_metadata(
		v.context(),
		v.DataHandle(),
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
