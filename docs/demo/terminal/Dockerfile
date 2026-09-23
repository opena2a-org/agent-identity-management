# Terminal recorder: the VHS image hackmyagent builds from docs/vhs/docker
# (VHS 0.12.0 with its encode-context regression patched), plus Python with
# venv and pip, running as the unprivileged user `demo` with HOME=/home/demo.
# The Python SDK is not baked in: the tape installs it in front of the viewer
# (a rehearsal installs the checkout's sdk/python under Hide instead).
#
# Build the base first, from a hackmyagent checkout at 3f698f5 or later:
#   docker build -t hma-vhs:0.33.0 <hackmyagent>/docs/vhs/docker
ARG BASE_IMAGE=hma-vhs:0.33.0
FROM ${BASE_IMAGE}
RUN apt-get update -qq \
 && apt-get install -y -qq --no-install-recommends python3 python3-venv python3-pip bsdutils >/dev/null \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --shell /bin/bash demo \
 && mkdir -p /work/out && chown -R demo:demo /work
USER demo
ENV HOME=/home/demo
ENV PYTHON_BASIC_REPL=1
ENV PYTHONUNBUFFERED=1
WORKDIR /home/demo
ENTRYPOINT ["vhs"]
