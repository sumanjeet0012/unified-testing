FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev iproute2 git && rm -rf /var/lib/apt/lists/*

# Install py-libp2p from the spec-compliance branch: the PyPI release predates
# the circuit-relay v2 + DCUtR/hole-punch fixes required for NAT interop.
# Re-pin to a release once merged upstream.
# NOTE: py-libp2p comes from a moving branch; touch this file to force
# a rebuild when only the upstream branch changed.
RUN pip install --no-cache-dir \
        "git+https://github.com/sumanjeet0012/py-libp2p.git@fix/relay-v2-spec-compliance" \
        redis multiaddr

COPY node.py run-peer.sh ./
RUN chmod +x run-peer.sh

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["/app/run-peer.sh"]
