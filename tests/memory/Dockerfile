# this must be openvds image in debug mode
ARG OPENVDS_IMAGE=openvdsdebug
FROM ${OPENVDS_IMAGE} AS openvds

FROM python:3.11-alpine

RUN apk --no-cache add \
    valgrind \
    g++ \
    gcc \
    make \
    cmake \
    boost-dev \
    util-linux-dev

COPY --from=openvds /open-vds/Dist/OpenVDS /open-vds

COPY ./tests/memory/requirements-dev.txt /tests/memory/requirements-dev.txt
RUN python -m pip install --upgrade pip
RUN pip install -r /tests/memory/requirements-dev.txt

COPY . .

ARG BUILD_DIR="build"
RUN cmake -S . -B "${BUILD_DIR}" -DCMAKE_BUILD_TYPE=Debug -DGTEST=OFF -DMEMORYTEST=ON -DBUILD_CCORE=ON -DCMAKE_PREFIX_PATH=/open-vds
RUN cmake --build "${BUILD_DIR}" || exit 1

ENV EXECUTABLE_PATH="/${BUILD_DIR}/tests/memorytests"
# openvds suppressions are presented in debug mode so it is easier to see the errors
# thus why openvds itself should be compiled in debug mode to match those errors
ENV VALGRIND_SUPPRESSION_PATH="/tests/memory/valgrind_alpine.supp, /tests/memory/openvds_debug.supp"

WORKDIR /tests
ENV PYTHONPATH="${PYTHONPATH}:/tests"
ENTRYPOINT ["python", "memory/memory.py"]
