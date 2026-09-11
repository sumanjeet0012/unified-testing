FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev git && rm -rf /var/lib/apt/lists/*

# Install py-libp2p from the spec-compliance branch: the PyPI release predates
# the AutoNAT spec fixes required for interop. Re-pin to a release once merged.
RUN pip install --no-cache-dir \
        "git+https://github.com/sumanjeet0012/py-libp2p.git@fix/relay-v2-spec-compliance" \
        redis multiaddr

COPY node.py .

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "node.py"]
