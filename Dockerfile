ARG OPENVDS_IMAGE=openvds
ARG ONESEISMIC_BASEIMAGE=docker.io/library/golang:1.22-alpine
FROM ${ONESEISMIC_BASEIMAGE} AS openvds
RUN apk --no-cache add \
    git \
    g++ \
    gcc \
    make \
    cmake \
    boost-dev \
    util-linux-dev \
    perl \
    libuv-dev \
    zlib-dev \
    openssl-dev

ARG OPENVDS_VERSION=3.4.5
WORKDIR /
RUN git clone --depth 1 --branch ${OPENVDS_VERSION} https://community.opengroup.org/osdu/platform/domain-data-mgmt-services/seismic/open-vds.git
WORKDIR /open-vds

# for performance reasons open-vds should be build and run with new openssl
# libssl is installed on alpine by default and would be used during runtime

# it is unclear if curl version makes a difference and which one is better
ARG BUILD_TYPE=Release
RUN cmake -S . \
    -B build \
    -DCMAKE_BUILD_TYPE=${BUILD_TYPE} \
    -DBUILD_JAVA=OFF \
    -DBUILD_PYTHON=OFF \
    -DBUILD_EXAMPLES=OFF \
    -DBUILD_TESTS=OFF \
    -DBUILD_DOCS=OFF \
    -DDISABLE_STRICT_WARNINGS=OFF \
    -DDISABLE_AZURESDKFORCPP_IOMANAGER=OFF \
    -DDISABLE_AWS_IOMANAGER=ON \
    -DDISABLE_GCP_IOMANAGER=ON \
    -DDISABLE_DMS_IOMANAGER=ON \
    -DDISABLE_AZURE_PRESIGNED_IOMANAGER=ON \
    -DDISABLE_CURL_IOMANAGER=ON \
    -DBUILD_UV=OFF \
    -DBUILD_ZLIB=OFF \
    -DBUILD_OPENSSL=OFF \
    -DBUILD_CURL=ON

RUN cmake --build build --config ${BUILD_TYPE} --target install -j 8 --verbose


FROM $OPENVDS_IMAGE AS openvds_snyk_monitor
WORKDIR /
RUN apk --no-cache add curl
RUN curl --compressed https://downloads.snyk.io/cli/stable/snyk-alpine -o snyk
RUN chmod +x ./snyk
WORKDIR /open-vds
ENTRYPOINT /snyk monitor --unmanaged --remote-repo-url=${SNYK_PROJECT_TAG}


FROM $OPENVDS_IMAGE AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
RUN go build -a ./...
RUN GOBIN=/tools go install github.com/swaggo/swag/cmd/swag@v1.16.3
RUN /tools/swag init --dir cmd/query,api/handlers,api/middleware,internal/core -g main.go --md docs


FROM builder AS tester
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
ARG LD_LIBRARY_PATH=/open-vds/Dist/OpenVDS/lib:$LD_LIBRARY_PATH

ENV CMAKE_PREFIX_PATH=$CMAKE_PREFIX_PATH:/open-vds/Dist/OpenVDS
RUN cmake -S . -B build
RUN cmake --build build
RUN ctest --test-dir build

ARG OPENVDS_AZURESDKFORCPP=1
RUN go test -race ./...


FROM builder AS static_analyzer
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
ARG STATICCHECK_VERSION="2023.1.7"
ARG LD_LIBRARY_PATH=/open-vds/Dist/OpenVDS/lib:$LD_LIBRARY_PATH
RUN apk --no-cache add curl
RUN curl \
    -L https://github.com/dominikh/go-tools/releases/download/${STATICCHECK_VERSION}/staticcheck_linux_amd64.tar.gz \
    -o staticcheck-${STATICCHECK_VERSION}.tar.gz
RUN ls && tar xf staticcheck-${STATICCHECK_VERSION}.tar.gz
RUN ./staticcheck/staticcheck ./...


FROM builder AS installer
ARG CGO_CPPFLAGS="-I/open-vds/Dist/OpenVDS/include"
ARG CGO_LDFLAGS="-L/open-vds/Dist/OpenVDS/lib"
ARG LD_LIBRARY_PATH=/open-vds/Dist/OpenVDS/lib:$LD_LIBRARY_PATH
RUN GOBIN=/server go install -a ./...

FROM ${ONESEISMIC_BASEIMAGE} AS runner
RUN apk --no-cache add \
    jemalloc-dev

WORKDIR /server
COPY --from=installer /open-vds/Dist/OpenVDS/lib/* /open-vds/
COPY --from=installer /server /server
COPY --from=installer /src/docs/index.html /server/docs/

RUN addgroup -S -g 1001 radix-non-root-group
RUN adduser -S -u 1001 -G radix-non-root-group radix-non-root-user
USER 1001

ENV LD_LIBRARY_PATH=/open-vds:$LD_LIBRARY_PATH
ENV OPENVDS_AZURESDKFORCPP=1
ENV LD_PRELOAD=/usr/lib/libjemalloc.so
ENTRYPOINT [ "/server/query" ]
