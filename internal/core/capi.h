#ifndef ONESEISMIC_API_CAPI_H
#define ONESEISMIC_API_CAPI_H

#include "ctypes.h"

#ifdef __cplusplus
extern "C" {
#endif

/** Return value status codes */
enum status_code {
    STATUS_OK = 0,
    STATUS_NULLPTR_ERROR,
    STATUS_RUNTIME_ERROR,
    STATUS_BAD_REQUEST
};

/** Carry additional context between caller and functions
 *
 * Any function that accepts a context as one of its input parameters can use
 * it to write additional information that might be of use to the caller. This
 * includes, but is not limited, to writing error messages into the context if
 * the function should fail. In that case the caller can call errmsg(Context*
 * ctx) to retrieve the error message.
 */
struct Context;
typedef struct Context Context;

/** Create a new context instance
 *
 * The returned instance must always be explicitly free'd by a
 * call to context_free().
 */
Context* context_new();

/** Free up the resources managed internally by the context */
int context_free(Context* ctx);

/** Read out the last error msg set on the context */
const char* errmsg(Context* ctx);

response response_create();
void response_delete(struct response*);

struct DataHandle;
typedef struct DataHandle DataHandle;

int single_datahandle_new(
    Context* ctx,
    const char* url,
    const char* credentials,
    DataHandle** ds_out
);

int double_datahandle_new(
    Context* ctx,
    const char* url_A,
    const char* credentials_A,
    const char* url_B,
    const char* credentials_B,
    enum binary_operator bin_operator,
    DataHandle** ds_out
);

int datahandle_free(Context* ctx, DataHandle* f);

struct RegularSurface;
typedef struct RegularSurface RegularSurface;

int regular_surface_new(
    Context* ctx,
    float* data,
    size_t nrows,
    size_t ncols,
    float xori,
    float yori,
    float xinc,
    float yinc,
    float rot,
    float fillvalue,
    RegularSurface** out
);

int regular_surface_free(
    Context* ctx,
    RegularSurface* surface
);

struct SurfaceBoundedSubVolume;
typedef struct SurfaceBoundedSubVolume SurfaceBoundedSubVolume;

int subvolume_new(
    Context* ctx,
    DataHandle* datahandle,
    RegularSurface* reference,
    RegularSurface* top,
    RegularSurface* bottom,
    SurfaceBoundedSubVolume** out
);

int subvolume_free(
    Context* ctx,
    SurfaceBoundedSubVolume* subvolume
);

int metadata(
    Context* ctx,
    DataHandle* datahandle,
    response* out
);

int slice(
    Context* ctx,
    DataHandle* datahandle,
    int lineno,
    enum axis_name direction,
    struct Bound* bounds,
    size_t nbounds,
    response* out
);

int slice_metadata(
    Context* ctx,
    DataHandle* datahandle,
    int lineno,
    enum axis_name direction,
    struct Bound* bounds,
    size_t nbounds,
    response* out
);

int fence(
    Context* ctx,
    DataHandle* datahandle,
    enum coordinate_system coordinate_system,
    const float* points,
    size_t npoints,
    enum interpolation_method interpolation_method,
    const float* fillValue,
    response* out
);

int fence_metadata(
    Context* ctx,
    DataHandle* datahandle,
    size_t npoints,
    response* out
);

int attribute_metadata(
    Context* ctx,
    DataHandle* datahandle,
    size_t nrows,
    size_t ncols,
    response* out
);

/** Attribute calculation
*
* Output buffer
* -------------
*
* Ideally we want to allocate n output buffers in Golang. One for every target
* attribute. Each buffer is passed through C with a void* and since we need N
* void* they should be passed as an array (void**).
* However Golang prohibits us from passing Go pointer to memory containing Go
* pointers (for memory safety reasons) [1].
*
* Thus this function instead accepts a single contiguous buffer with enough
* room for all attributes. The total buffer size should be mapsize *
* nattributes, where mapsize is the number of bytes for a single attribute
* result.
*
* [1] https://pkg.go.dev/cmd/cgo#hdr-Passing_pointers
*/
int attribute(
    Context* ctx,
    DataHandle* datahandle,
    SurfaceBoundedSubVolume* src_subvolume,
    enum interpolation_method interpolation_method,
    enum attribute* attributes,
    size_t nattributes,
    float stepsize,
    size_t from,
    size_t to,
    void* out
);

int align_surfaces(
    Context* ctx,
    RegularSurface* primary,
    RegularSurface* secondary,
    RegularSurface* aligned,
    int* primary_is_top
);

#ifdef __cplusplus
}
#endif
#endif // ONESEISMIC_API_CAPI_H
