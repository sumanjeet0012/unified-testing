FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev iproute2 git && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
        "git+https://github.com/libp2p/py-libp2p.git@main" \
        redis multiaddr

COPY node.py run-peer.sh ./
RUN chmod +x run-peer.sh

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["/app/run-peer.sh"]
