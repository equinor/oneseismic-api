#include "vds.h"

#include <array>
#include <algorithm>
#include <string>
#include <stdexcept>
#include <memory>
#include <cmath>

#include "nlohmann/json.hpp"

#include <OpenVDS/OpenVDS.h>
#include <OpenVDS/KnownMetadata.h>
#include <OpenVDS/IJKCoordinateTransformer.h>

#include "attribute.hpp"
#include "axis.hpp"
#include "boundingbox.hpp"
#include "datahandle.hpp"
#include "direction.hpp"
#include "metadatahandle.hpp"
#include "regularsurface.hpp"
#include "subvolume.hpp"

void response_delete(struct response* buf) {
    if (!buf)
        return;

    delete[] buf->data;
    delete[] buf->err;
    *buf = response {};
}

std::string fmtstr(OpenVDS::VolumeDataFormat format) {
    /*
     * We always request data in OpenVDS::VolumeDataFormat::Format_R32 format
     * as this seems to be intended way when working with openvds [1].
     * Thus users will always get data returned as f4.
     *
     * We also assume that server code is run on a little-endian machine.
     *
     * [1] https://community.opengroup.org/osdu/platform/domain-data-mgmt-services/seismic/open-vds/-/issues/156#note_165511
     */
    switch (format) {
        case OpenVDS::VolumeDataFormat::Format_R32: return "<f4";
        default: {
            throw std::runtime_error("unsupported VDS format type");
        }
    }
}

struct response to_response(nlohmann::json const& metadata) {
    auto const dump = metadata.dump();
    std::unique_ptr< char[] > tmp(new char[dump.size()]);
    std::copy(dump.begin(), dump.end(), tmp.get());
    return response{tmp.release(), nullptr, dump.size()};
}

struct response to_response(std::unique_ptr< char[] > data, std::int64_t const size) {
    /* The data should *not* be free'd on success, as it's returned to CGO */
    return response{data.release(), nullptr, static_cast<unsigned long>(size)};
}

struct response to_response(std::exception const& e) {
    std::size_t size = std::char_traits<char>::length(e.what()) + 1;

    std::unique_ptr<char[]> msg(new char[size]);
    std::copy(e.what(), e.what() + size, msg.get());
    return response{nullptr, msg.release(), 0};
}

/*
 * Unit validation of Z-slices
 *
 * Verify that the units of the VDS' Z axis matches the requested slice axis.
 * E.g. a Time slice is only valid if the units of the Z-axis in the VDS is
 * "Seconds" or "Milliseconds"
 */
bool unit_validation(axis_name ax, std::string const& zunit) {
    /* Define some convenient lookup tables for units */
    static const std::array< const char*, 3 > depthunits = {
        OpenVDS::KnownUnitNames::Meter(),
        OpenVDS::KnownUnitNames::Foot(),
        OpenVDS::KnownUnitNames::USSurveyFoot()
    };

    static const std::array< const char*, 2 > timeunits = {
        OpenVDS::KnownUnitNames::Millisecond(),
        OpenVDS::KnownUnitNames::Second()
    };

    static const std::array< const char*, 1 > sampleunits = {
        OpenVDS::KnownUnitNames::Unitless(),
    };

    auto isoneof = [zunit](const char* x) {
        return !std::strcmp(x, zunit.c_str());
    };

    switch (ax) {
        case I:
        case J:
        case K:
        case INLINE:
        case CROSSLINE:
            return true;
        case DEPTH:
            return std::any_of(depthunits.begin(), depthunits.end(), isoneof);
        case TIME:
            return std::any_of(timeunits.begin(), timeunits.end(), isoneof);
        case SAMPLE:
            return std::any_of(sampleunits.begin(), sampleunits.end(), isoneof);
        default: {
            throw std::runtime_error("Unhandled axis");
        }
    }
};

nlohmann::json json_axis(
    Axis const& axis
) {
    nlohmann::json doc;
    doc = {
        { "annotation", axis.name()     },
        { "min",        axis.min()      },
        { "max",        axis.max()      },
        { "samples",    axis.nsamples() },
        { "unit",       axis.unit()     },
    };
    return doc;
}

struct response fetch_slice(
    std::string url,
    std::string credentials,
    Direction const direction,
    int lineno
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    Axis const& axis = metadata.get_axis(direction);
    std::string zunit = metadata.sample().unit();
    if (not unit_validation(direction.name(), zunit)) {
        std::string msg = "Unable to use " + direction.to_string();
        msg += " on cube with depth units: " + zunit;
        throw std::runtime_error(msg);
    }

    SubVolume bounds(metadata);
    bounds.set_slice(axis, lineno, direction.coordinate_system());

    std::int64_t const size = handle.subvolume_buffer_size(bounds);

    std::unique_ptr< char[] > data(new char[size]);
    handle.read_subvolume(data.get(), size, bounds);

    return to_response(std::move(data), size);
}

struct response fetch_slice_metadata(
    std::string url,
    std::string credentials,
    Direction const direction
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    nlohmann::json meta;
    meta["format"] = fmtstr(DataHandle::format());

    /*
     * SEGYImport always writes annotation 'Sample' for axis K. We, on the
     * other hand, decided that we base the valid input direction on the units
     * of said axis. E.g. ms/s -> Time, etc. This leads to an inconsistency
     * between what we require as input for axis K and what we return as
     * metadata. In the ms/s case we require the input to be asked for in axis
     * 'Time', but the return metadata can potentially say 'Sample'.
     *
     * TODO: Either revert the 'clever' unit validation, or patch the
     * K-annotation here. IMO the later is too clever for it's own good and
     * would be quite suprising for people that use this API in conjunction
     * with the OpenVDS library.
     */
    Axis const& inline_axis = metadata.iline();
    Axis const& crossline_axis = metadata.xline();
    Axis const& sample_axis = metadata.sample();

    if (direction.is_iline()) {
        meta["x"] = json_axis(sample_axis);
        meta["y"] = json_axis(crossline_axis);
    } else if (direction.is_xline()) {
        meta["x"] = json_axis(sample_axis);
        meta["y"] = json_axis(inline_axis);
    } else if (direction.is_sample()) {
        meta["x"] = json_axis(crossline_axis);
        meta["y"] = json_axis(inline_axis);
    } else {
        throw std::runtime_error("Unhandled direction");
    }

    return to_response(meta);
}

struct response fetch_fence(
    const std::string& url,
    const std::string& credentials,
    enum coordinate_system coordinate_system,
    const float* coordinates,
    size_t npoints,
    enum interpolation_method interpolation_method
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    std::unique_ptr< voxel[] > coords(new voxel[npoints]{{0}});

    auto coordinate_transformer = metadata.coordinate_transformer();
    auto transform_coordinate = [&] (const float x, const float y) {
        switch (coordinate_system) {
            case INDEX:
                return OpenVDS::Vector<double, 3> {x, y, 0};
            case ANNOTATION:
                return coordinate_transformer.AnnotationToIJKPosition({x, y, 0});
            case CDP:
                return coordinate_transformer.WorldToIJKPosition({x, y, 0});
            default: {
                throw std::runtime_error("Unhandled coordinate system");
            }
        }
    };

    Axis const& inline_axis = metadata.iline();
    Axis const& crossline_axis = metadata.xline();

    for (size_t i = 0; i < npoints; i++) {
        const float x = *(coordinates++);
        const float y = *(coordinates++);

        auto coordinate = transform_coordinate(x, y);

        auto validate_boundary = [&] (const int voxel, Axis const& axis) {
            const auto min = -0.5;
            const auto max = axis.nsamples() - 0.5;
            if(coordinate[voxel] < min || coordinate[voxel] >= max) {
                const std::string coordinate_str =
                    "(" +std::to_string(x) + "," + std::to_string(y) + ")";
                throw std::runtime_error(
                    "Coordinate " + coordinate_str + " is out of boundaries "+
                    "in dimension "+ std::to_string(voxel)+ "."
                );
            }
        };

        validate_boundary(0, inline_axis);
        validate_boundary(1, crossline_axis);

        /* OpenVDS' transformers and OpenVDS data request functions have
         * different definition of where a datapoint is. E.g. A transformer
         * (To voxel or ijK) will return (0,0,0) for the first sample in
         * the cube. The request functions on the other hand assumes the
         * data is located in the center of a voxel. I.e. that the first
         * sample is at (0.5, 0.5, 0.5). This is a *VERY* sharp edge in the
         * OpenVDS API and borders on a bug. It means we cannot directly
         * use the output from the transformers as input to the request
         * functions.
         */
        coordinate[0] += 0.5;
        coordinate[1] += 0.5;

        coords[i][   inline_axis.dimension()] = coordinate[0];
        coords[i][crossline_axis.dimension()] = coordinate[1];
    }

    std::int64_t const size = handle.traces_buffer_size(npoints);

    std::unique_ptr< char[] > data(new char[size]);

    handle.read_traces(
        data.get(),
        size,
        coords.get(),
        npoints,
        interpolation_method
    );

    return to_response(std::move(data), size);
}

struct response fetch_fence_metadata(
    std::string url,
    std::string credentials,
    size_t npoints
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    nlohmann::json meta;
    Axis const& sample_axis = metadata.sample();
    meta["shape"] = nlohmann::json::array({npoints, sample_axis.nsamples() });
    meta["format"] = fmtstr(DataHandle::format());

    return to_response(meta);
}

struct response metadata(
    const std::string& url,
    const std::string& credentials
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    nlohmann::json meta;

    meta["crs"] = metadata.crs();
    meta["inputFileName"] = metadata.input_filename();

    auto bbox = metadata.bounding_box();
    meta["boundingBox"]["ij"]   = bbox.index();
    meta["boundingBox"]["cdp"]  = bbox.world();
    meta["boundingBox"]["ilxl"] = bbox.annotation();


    Axis const& inline_axis = metadata.iline();
    meta["axis"].push_back(json_axis(inline_axis));

    Axis const& crossline_axis = metadata.xline();
    meta["axis"].push_back(json_axis(crossline_axis));

    Axis const& sample_axis = metadata.sample();
    meta["axis"].push_back(json_axis(sample_axis));

    return to_response(meta);
}

/**
 * For every index in 'novals', write n successive floats with value
 * 'fillvalue' to dst. Where n is 'vertical_size'.
 */
void write_fillvalue(
    char * dst,
    std::vector< std::size_t > const& novals,
    std::size_t vertical_size,
    float fillvalue
) {
    std::vector< float > fill(vertical_size, fillvalue);
    std::for_each(novals.begin(), novals.end(), [&](std::size_t i) {
        std::memcpy(
            dst + i * sizeof(float),
            fill.data(),
            fill.size() * sizeof(float)
        );
    });
}

struct response fetch_horizon(
    std::string const&        url,
    std::string const&        credentials,
    RegularSurface            surface,
    float                     fillvalue,
    float                     above,
    float                     below,
    enum interpolation_method interpolation
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();
    auto transform = metadata.coordinate_transformer();

    auto const& iline  = metadata.iline ();
    auto const& xline  = metadata.xline();
    auto const& sample = metadata.sample();

    auto vertical = VerticalWindow(above, below, sample.stride());
    vertical.squeeze();

    std::size_t const nsamples = surface.size() * vertical.size();

    std::unique_ptr< voxel[] > samples(new voxel[nsamples]{{0}});

    auto inrange = [](Axis const& axis, double const voxel) {
        return (0 <= voxel) and (voxel < axis.nsamples());
    };

    /** Missing input samples (marked by fillvalue) and out of bounds samples
     *
     * To not overcomplicate things for ourselfs (and the caller) we guarantee
     * that the output amplitude map is exacty the same dimensions as the input
     * height map (horizon). That gives us 2 cases to explicitly handle:
     *
     * 1) If a sample (region of samples) in the input horizon is marked as
     * missing by the fillvalue then the fillvalue is used in that position in
     * the output array too:
     *
     *      input[n][m] == fillvalue => output[n][m] == fillvalue
     *
     * 2) If a sample (or region of samples) in the input horizon is out of
     * bounds in the horizontal plane, the output sample is populated by the
     * fillvalue.
     *
     * openvds provides no options to handle these cases and to keep the output
     * buffer aligned with the input we cannot drop samples that satisfy 1) or
     * 2). Instead we let openvds read a dummy voxel  ({0, 0, 0, 0, 0, 0}) and
     * keep track of the indices. After openvds is done we copy in the
     * fillvalue.
     *
     * The overhead of this approach is that we overfetch (at most) one one
     * chunk and we need an extra loop over output array.
     */
    std::vector< std::size_t > noval_indicies;

    std::size_t i = 0;
    for (int row = 0; row < surface.nrows(); row++) {
        for (int col = 0; col < surface.ncols(); col++) {
            float const depth = surface.value(row, col);
            if (depth == fillvalue) {
                noval_indicies.push_back(i);
                i += vertical.size();
                continue;
            }

            auto const cdp = surface.coordinate(row, col);

            auto ij = transform.WorldToIJKPosition({cdp.x, cdp.y, 0});
            auto k  = transform.AnnotationToIJKPosition({0, 0, depth});

            /* OpenVDS' transformers and OpenVDS data request functions have
             * different definition of where a datapoint is. E.g. A transformer
             * (To voxel or ijK) will return (0,0,0) for the first sample in
             * the cube. The request functions on the other hand assumes the
             * data is located in the center of a voxel. I.e. that the first
             * sample is at (0.5, 0.5, 0.5). This is a *VERY* sharp edge in the
             * OpenVDS API and borders on a bug. It means we cannot directly
             * use the output from the transformers as input to the request
             * functions.
             */
            ij[0] += 0.5;
            ij[1] += 0.5;
             k[2] += 0.5;

            if (not inrange(iline, ij[0]) or not inrange(xline, ij[1])) {
                noval_indicies.push_back(i);
                i += vertical.size();
                continue;
            }

            double top    = k[2] - vertical.nsamples_above();
            double bottom = k[2] + vertical.nsamples_below();
            if (not inrange(sample, top) or not inrange(sample, bottom)) {
                throw std::runtime_error(
                    "Vertical window is out of vertical bounds at"
                    " row: " + std::to_string(row) +
                    " col:" + std::to_string(col) +
                    ". Request: [" + std::to_string(bottom) +
                    ", " + std::to_string(top) +
                    "]. Seismic bounds: [" + std::to_string(sample.min())
                    + ", " +std::to_string(sample.max()) + "]"
                );
            }
            for (double cur_depth = top; cur_depth <= bottom; cur_depth++) {
                samples[i][  iline.dimension() ] = ij[0];
                samples[i][  xline.dimension() ] = ij[1];
                samples[i][ sample.dimension() ] = cur_depth;
                ++i;
            }
        }
    }

    auto const size = handle.samples_buffer_size(nsamples);

    std::unique_ptr< char[] > buffer(new char[size]());
    handle.read_samples(
        buffer.get(),
        size,
        samples.get(),
        nsamples,
        interpolation
    );

    write_fillvalue(buffer.get(), noval_indicies, vertical.size(), fillvalue);

    return to_response(std::move(buffer), size);
}

struct response calculate_attribute(
    DataHandle& handle,
    Horizon const& horizon,
    enum attribute* attributes,
    size_t nattributes
) {
    using namespace attributes;

    MetadataHandle const& metadata = handle.get_metadata();

    auto const& vertical = horizon.vertical();
    std::size_t index = vertical.nsamples_above();
    std::size_t vsize = vertical.size();

    auto const& surface = horizon.surface();
    std::size_t size = surface.size() * sizeof(float);

    std::unique_ptr< char[] > buffer(new char[size * nattributes]());

    std::vector< Attribute > attrs;
    for (int i = 0; i < nattributes; ++i) {
        char* dst = buffer.get() + (i * size);

        switch (*attributes) {
            case VALUE:  { attrs.push_back( Value(dst, size, index) ); break; }
            case MIN:    { attrs.push_back(   Min(dst, size) )       ; break; }
            case MAX:    { attrs.push_back(   Max(dst, size) )       ; break; }
            case MEAN:   { attrs.push_back(  Mean(dst, size, vsize) ); break; }
            case RMS:    { attrs.push_back(   Rms(dst, size, vsize) ); break; }
            default:
                throw std::runtime_error("Attribute not implemented");
        }
        ++attributes;
    }

    horizon.calc_attributes(attrs);
    return to_response(std::move(buffer), size * nattributes);
}

struct response fetch_attribute_metadata(
    std::string const& url,
    std::string const& credentials,
    std::size_t nrows,
    std::size_t ncols
) {
    DataHandle handle(url, credentials);
    MetadataHandle const& metadata = handle.get_metadata();

    nlohmann::json meta;
    meta["shape"] = nlohmann::json::array({nrows, ncols});
    meta["format"] = fmtstr(DataHandle::format());

    return to_response(meta);
}

struct response slice(
    const char* vds,
    const char* credentials,
    int lineno,
    axis_name ax
) {
    std::string cube(vds);
    std::string cred(credentials);
    Direction const direction(ax);

    try {
        return fetch_slice(cube, cred, direction, lineno);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response slice_metadata(
    const char* vds,
    const char* credentials,
    axis_name ax
) {
    std::string cube(vds);
    std::string cred(credentials);
    Direction const direction(ax);

    try {
        return fetch_slice_metadata(cube, cred, direction);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response fence(
    const char* vds,
    const char* credentials,
    enum coordinate_system coordinate_system,
    const float* coordinates,
    size_t npoints,
    enum interpolation_method interpolation_method
) {
    std::string cube(vds);
    std::string cred(credentials);

    try {
        return fetch_fence(
            cube, cred, coordinate_system, coordinates, npoints,
            interpolation_method);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response fence_metadata(
    const char* vds,
    const char* credentials,
    size_t npoints
) {
    std::string cube(vds);
    std::string cred(credentials);

    try {
        return fetch_fence_metadata(cube, cred, npoints);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response metadata(
    const char* vds,
    const char* credentials
) {
    try {
        std::string cube(vds);
        std::string cred(credentials);
        return metadata(cube, cred);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response horizon(
    const char*  vdspath,
    const char* credentials,
    const float* data,
    size_t nrows,
    size_t ncols,
    float xori,
    float yori,
    float xinc,
    float yinc,
    float rot,
    float fillvalue,
    float above,
    float below,
    enum interpolation_method interpolation
) {
    try {
        std::string cube(vdspath);
        std::string cred(credentials);

        RegularSurface surface{data, nrows, ncols, xori, yori, xinc, yinc, rot};

        return fetch_horizon(
            cube,
            cred,
            surface,
            fillvalue,
            above,
            below,
            interpolation
        );
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response attribute_metadata(
    const char*  vdspath,
    const char* credentials,
    size_t nrows,
    size_t ncols
) {
    try {
        std::string cube(vdspath);
        std::string cred(credentials);

        return fetch_attribute_metadata(cube, cred, nrows, ncols);
    } catch (const std::exception& e) {
        return to_response(e);
    }
}

struct response attribute(
    const char* vdspath,
    const char* credentials,
    const float* surface_data,
    size_t nrows,
    size_t ncols,
    float xori,
    float yori,
    float xinc,
    float yinc,
    float rot,
    float fillvalue,
    const char* horizon_data,
    float above,
    float below,
    enum attribute* attributes,
    size_t nattributes
) {
    try {
        DataHandle handle(vdspath, credentials);
        MetadataHandle const& metadata = handle.get_metadata();
        auto const& sample = metadata.sample();

        auto window = VerticalWindow(above, below, sample.stride());
        window.squeeze();

        RegularSurface surface(
            surface_data,
            nrows,
            ncols,
            xori,
            yori,
            xinc,
            yinc,
            rot
        );

        Horizon horizon(
            (float*)horizon_data,
            surface,
            window,
            fillvalue
        );

        return calculate_attribute(
            handle,
            horizon,
            attributes,
            nattributes
        );
    } catch (const std::exception& e) {
        return to_response(e);
    }
}
