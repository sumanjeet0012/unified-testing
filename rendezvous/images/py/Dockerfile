FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev git && rm -rf /var/lib/apt/lists/*

# Pinned to the /p2p-strip fix (libp2p/py-libp2p#1536, commit 00da5aea) until it
# lands in a PyPI release, then revert to 'libp2p==<release>'.
RUN pip install --no-cache-dir 'libp2p @ git+https://github.com/sumanjeet0012/py-libp2p.git@00da5aea69c1914b18368f4a1da5711929f27682' redis multiaddr

COPY node.py .

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "node.py"]
