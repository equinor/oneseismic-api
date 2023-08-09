#include "metadatahandle.hpp"

#include <stdexcept>
#include <list>
#include <utility>

#include <OpenVDS/KnownMetadata.h>
#include "axis.hpp"
#include "boundingbox.hpp"
#include "direction.hpp"

MetadataHandle::MetadataHandle(OpenVDS::VolumeDataLayout const * const layout)
    : m_layout(layout),
    m_iline(Axis(layout,  get_dimension({axis_name::INLINE}))),
    m_xline(Axis(layout,  get_dimension({axis_name::CROSSLINE}))),
    m_sample(Axis(layout, get_dimension({axis_name::SAMPLE, axis_name::DEPTH, axis_name::TIME})))
{
    this->dimension_validation();
}

void MetadataHandle::dimension_validation() const {
    if (this->m_layout->GetDimensionality() != 3) {
        throw std::runtime_error(
            "Unsupported VDS, expected 3 dimensions, got " +
            std::to_string(this->m_layout->GetDimensionality())
        );
    }
}

int
MetadataHandle::get_dimension(std::vector< enum axis_name > names) const {

    for(auto i=0; i < this->m_layout->GetDimensionality(); i++){
        std::string dimensionName = this->m_layout->GetDimensionName(i);
        for(auto name : names){
            auto direction = Direction(name);
            if (dimensionName == direction.to_string()){
                return i;
            }
        }
    }
    throw std::runtime_error(
        "Unsupported axis in VDS file, expected (Sample|Time|Depth, Crossline, Inline) but got " +
        std::to_string(this->m_layout->GetDimensionality())
    );
}

Axis MetadataHandle::iline() const noexcept (true) {
    return this->m_iline;
}

Axis MetadataHandle::xline() const noexcept (true) {
    return this->m_xline;
}

Axis MetadataHandle::sample() const noexcept (true) {
    return this->m_sample;
}

BoundingBox MetadataHandle::bounding_box() const noexcept (true) {
    return BoundingBox(
        this->iline().nsamples(),
        this->xline().nsamples(),
        this->coordinate_transformer()
    );
}

std::string MetadataHandle::crs() const noexcept (true) {
    auto const crs = OpenVDS::KnownMetadata::SurveyCoordinateSystemCRSWkt();
    return this->m_layout->GetMetadataString(crs.GetCategory(), crs.GetName());
}

std::string MetadataHandle::input_filename() const noexcept (true) {
    auto const disp_name = OpenVDS::KnownMetadata::ImportInformationInputFileName();
    return this->m_layout->GetMetadataString(disp_name.GetCategory(), disp_name.GetName());
}

std::string MetadataHandle::import_time_stamp() const noexcept (true) {
    auto const time_stamp = OpenVDS::KnownMetadata::ImportInformationImportTimeStamp();
    return this->m_layout->GetMetadataString(time_stamp.GetCategory(), time_stamp.GetName());
}

Axis MetadataHandle::get_axis(
    Direction const direction
) const noexcept (false) {
    if      (direction.is_iline())  return this->iline();
    else if (direction.is_xline())  return this->xline();
    else if (direction.is_sample()) return this->sample();

    throw std::runtime_error("Unhandled axis");
}

OpenVDS::IJKCoordinateTransformer MetadataHandle::coordinate_transformer()
    const noexcept (true)
{
    return OpenVDS::IJKCoordinateTransformer(this->m_layout);
}
