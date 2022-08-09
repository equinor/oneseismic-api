FROM golang:1.18-alpine as openvds
RUN apk --no-cache add \
    git \
    g++ \
    gcc \
    make \
    cmake \
    curl-dev \
    boost-dev \
    libxml2-dev \
    libuv-dev \
    util-linux-dev

WORKDIR /
RUN git clone https://community.opengroup.org/erha/open-vds.git
WORKDIR /open-vds
RUN git checkout enable-cache

RUN cmake -S . \
    -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_JAVA=OFF \
    -DBUILD_PYTHON=OFF \
    -DBUILD_EXAMPLES=OFF \
    -DBUILD_TESTS=OFF \
    -DBUILD_DOCS=OFF \
    -DDISABLE_AWS_IOMANAGER=ON \
    -DDISABLE_AZURESDKFORCPP_IOMANAGER=OFF \
    -DDISABLE_GCP_IOMANAGER=ON \
    -DDISABLE_DMS_IOMANAGER=OFF \
    -DDISABLE_STRICT_WARNINGS=OFF
RUN cmake --build build   --config Release  --target install  -j 8 --verbose


FROM openvds as builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
RUN go build -a ./...


FROM builder as tester
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
ARG LD_LIBRARY_PATH=/open-vds/Dist/OpenVDS/lib:$LD_LIBRARY_PATH
ARG OPENVDS_AZURESDKFORCPP=1
RUN go test -race ./...


FROM builder as installer
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
ARG LD_LIBRARY_PATH=/open-vds/Dist/OpenVDS/lib:$LD_LIBRARY_PATH
RUN GOBIN=/server go install -a ./...


FROM golang:1.18-alpine as runner
RUN apk --no-cache add \
    g++ \
    gcc \
    libuv \
    libcurl \
    libxml2 \
    libuuid \
    boost-log

COPY --from=installer /open-vds/Dist/OpenVDS/lib/* /open-vds/
COPY --from=installer /server /server

RUN addgroup -S -g 1001 radix-non-root-group
RUN adduser -S -u 1001 -G radix-non-root-group radix-non-root-user
USER 1001

ENV LD_LIBRARY_PATH=/open-vds:$LD_LIBRARY_PATH
ENV OPENVDS_AZURESDKFORCPP=1
ENTRYPOINT [ "/server/query" ]
