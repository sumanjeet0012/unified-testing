FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev && rm -rf /var/lib/apt/lists/*

COPY py-libp2p /app/py-libp2p
RUN pip install --no-cache-dir /app/py-libp2p redis multiaddr

COPY node.py .

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "node.py"]
