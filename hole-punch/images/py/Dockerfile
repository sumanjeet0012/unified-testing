FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev iproute2 && rm -rf /var/lib/apt/lists/*

# TEMPORARY local-iteration wiring: install py-libp2p from the working tree
# at ../py-libp2p (fix/relay-v2-spec-compliance) instead of PyPI until the
# upstream fix merges. REVERT before committing.
COPY py-libp2p/ ./py-libp2p/
RUN pip install --no-cache-dir ./py-libp2p redis multiaddr

COPY node.py run-peer.sh ./
RUN chmod +x run-peer.sh

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["/app/run-peer.sh"]
