#ifndef SEISMICHANDLE_H
#define SEISMICHANDLE_H

#include <string>

#include <OpenVDS/OpenVDS.h>

#include "boundingbox.h"
#include "seismicaxismapping.h"
#include "vds.h"

enum class Channel : int {
    Sample = 0,
    Default = Sample
};

enum class LevelOfDetail : int {
    Level0 = 0,
    Default = Level0
};


class SeismicHandle {
public:
    SeismicHandle(
        const std::string    url,
        const std::string    connection,
        const Channel        default_channel,
        const LevelOfDetail  default_lod,
        const std::unique_ptr<SeismicAxisMap> axis_map
    );

    /// @brief  Maps from our Axis to a VDS axisDescriptor.
    /// @param axis
    /// @return
    OpenVDS::VolumeDataAxisDescriptor get_axis(Axis axis) const;
    BoundingBox get_bounding_box() const;
    std::string get_crs() const;
    std::string get_format(Channel ch = Channel::Default) const;

protected:
    class SeismicValidator {
        static constexpr int expected_dimensionality_{3};
        public:
        void validate(const SeismicHandle& vds_handle);
     };

    OpenVDS::ScopedVDSHandle handle_;
    OpenVDS::VolumeDataAccessManager access_manager_;
    OpenVDS::VolumeDataLayout const * layout_;
    std::unique_ptr<SeismicAxisMap> axis_map_;
    const Channel default_channel_;
    const LevelOfDetail default_lod_;
};

#endif /* SEISMICHANDLE_H */
