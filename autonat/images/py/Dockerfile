FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev && rm -rf /var/lib/apt/lists/*

# TEMPORARY local-iteration wiring: install py-libp2p from the working tree
# (spec-compliant AutoNAT lives on fix/relay-v2-spec-compliance, unreleased).
# REVERT to 'libp2p==<release>' before committing.
COPY py-libp2p/ ./py-libp2p/
RUN pip install --no-cache-dir ./py-libp2p redis multiaddr

COPY node.py .

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "node.py"]
