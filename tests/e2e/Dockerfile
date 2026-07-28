FROM python:3.14-slim@sha256:cea0e6040540fb2b965b6e7fb5ffa00871e632eef63719f0ea54bca189ce14a6 AS e2etest

COPY ./tests/e2e/requirements-dev.txt /tests/e2e/requirements-dev.txt
RUN python -m pip install --upgrade pip
RUN pip install -r /tests/e2e/requirements-dev.txt

COPY ./tests/utils /tests/utils
COPY ./tests/e2e /tests/e2e

WORKDIR /tests
ENTRYPOINT ["python","-m", "pytest", "-s", "-rsx", "e2e"]
