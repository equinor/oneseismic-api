#ifndef ONESEISMIC_API_CTYPES_H
#define ONESEISMIC_API_CTYPES_H
#include <stdlib.h>

struct response {
    char*         data;
    unsigned long size;
};
typedef struct response response;

enum axis_name {
    I         = 0,
    J         = 1,
    K         = 2,
    INLINE    = 3,
    CROSSLINE = 4,
    DEPTH     = 5,
    TIME      = 6,
    SAMPLE    = 7,
};

enum coordinate_system {
    INDEX      = 0,
    ANNOTATION = 1,
    CDP        = 2,
};

enum binary_operator {
    INVALID_OPERATOR = 0,
    NO_OPERATOR      = 1,
    ADDITION         = 2,
    SUBTRACTION      = 3,
    MULTIPLICATION   = 4,
    DIVISION         = 5,
};

enum interpolation_method {
    NEAREST,
    LINEAR,
    CUBIC,
    ANGULAR,
    TRIANGULAR
};

enum attribute {
    VALUE,
    MIN,
    MINAT,
    MAX,
    MAXAT,
    MAXABS,
    MAXABSAT,
    MEAN,
    MEANABS,
    MEANPOS,
    MEANNEG,
    MEDIAN,
    RMS,
    VAR,
    SD,
    SUMPOS,
    SUMNEG
};

struct Bound {
    int lower;
    int upper;
    enum axis_name name;
};

#endif // ONESEISMIC_API_CTYPES_H
