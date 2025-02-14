FROM python:3.10-slim
RUN apt-get update && apt-get install -y \
    curl \
    gettext-base

ARG k6_version=0.49.0
RUN mkdir k6
RUN curl \
    -L https://github.com/grafana/k6/releases/download/v${k6_version}/k6-v${k6_version}-linux-amd64.tar.gz \
    -o k6-${k6_version}.tar.gz
RUN tar xzf k6-${k6_version}.tar.gz -C k6 --strip-components=1

ENV PATH="$PATH:k6"

ARG prometheus_version=2.50.1
RUN mkdir prometheus
RUN curl \
    -L https://github.com/prometheus/prometheus/releases/download/v${prometheus_version}/prometheus-${prometheus_version}.linux-amd64.tar.gz \
    -o prometheus-${prometheus_version}.tar.gz
RUN tar xzf prometheus-${prometheus_version}.tar.gz -C prometheus --strip-components=1

ENV PATH="$PATH:prometheus"

COPY ./tests/utils /tests/utils
COPY ./tests/performance /tests/performance
RUN chmod +x /tests/performance/execute.sh
RUN python -m pip install -r /tests/performance/requirements-dev.txt
ENV PYTHONPATH="${PYTHONPATH}:/tests"
