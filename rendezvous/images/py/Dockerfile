# syntax=docker/dockerfile:1
FROM python:3.12-slim-bookworm

WORKDIR /app

# Cache apt archives and package lists so apt-get is fast and cached across builds
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends build-essential libgmp-dev

# Use Docker volume cache for pip downloads and wheels
COPY py-libp2p/pyproject.toml /app/pyproject.toml
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install redis multiaddr && \
    pip install .

COPY py-libp2p /app/py-libp2p
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-deps -e /app/py-libp2p

COPY node.py .

ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python", "node.py"]
