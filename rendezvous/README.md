# libp2p Rendezvous Protocol Interoperability Tests

Bash-driven interoperability tests for libp2p Rendezvous Protocol implementations (`/rendezvous/1.0.0`). The suite mirrors the overall design of [`transport/`](../transport/README.md) and [`kad-dht/`](../kad-dht/README.md): Docker Compose per test, a shared Redis instance for coordination, and a generated test matrix from `images.yaml`.

## Overview

Each test runs **three containers** in distinct roles:

| Role | Purpose |
|------|---------|
| **server** | Runs the Rendezvous Point service (`/rendezvous/1.0.0`); publishes its multiaddr to Redis and serves registration and discovery requests |
| **registrant** | Connects to the Rendezvous server, registers itself under a test-specific namespace with TTL, and listens for incoming ping requests |
| **discoverer** | Connects to the Rendezvous server, queries the namespace to discover the registrant, connects directly to the discovered registrant peer, and exchanges a ping/pong stream |

The goal is to verify that implementations can interoperate across the full server → register → discover → direct dial path, across different language stacks.

**Current implementations** (see [`images.yaml`](images.yaml)):

- **go** — go-libp2p-rendezvous (`rendezvous-go`)
- **py** — py-libp2p (`rendezvous-py`)

## Test Naming

Test IDs follow:

```
{server}_x_{registrant}_x_{discoverer}
```

The `_x_` separator is literal text. Each segment is an implementation id from `images.yaml`.

**Example:** `go_x_py_x_go`

| Segment | Role | Implementation |
|---------|------|----------------|
| 1st | server | `go` |
| 2nd | registrant | `py` |
| 3rd | discoverer | `go` |

This test verifies: *Can a Go node discover a Python registrant through a Go rendezvous server, and successfully dial that Python node?*

## Test Matrix Size

`lib/generate-tests.sh` builds the **full permutation** of server × registrant × discoverer for every implementation in `images.yaml`:

```
number of tests = N³
```

where `N` is the number of implementations.

| Implementations | Tests |
|-----------------|-------|
| 2 (`go`, `py`) | 8 |
| 3 (e.g. + `rust`) | 27 |
| 4 (e.g. + `js`) | 64 |

### All 8 Permutations Tested:

1. `go_x_go_x_go`
2. `go_x_go_x_py`
3. `go_x_py_x_go`
4. `go_x_py_x_py`
5. `py_x_go_x_go`
6. `py_x_go_x_py`
7. `py_x_py_x_go`
8. `py_x_py_x_py`

## Quick Start

### Run the full suite

```bash
cd rendezvous
./run.sh
```

### Run a subset of tests

```bash
# Run single test
./run.sh --test-select "go_x_py_x_go"

# Run all tests with Python as discoverer
./run.sh --test-select "*_x_*_x_py"

# Skip homogeneous tests
./run.sh --test-ignore "go_x_go_x_go|py_x_py_x_py"
```

### Inspect the matrix without running

```bash
./run.sh --list-tests
```

### Build images only

```bash
./run.sh --force-image-rebuild
```

## Directory Structure

```
rendezvous/
├── README.md
├── images.yaml              # Implementation definitions and image names
├── run.sh                   # Entrypoint test runner
├── images/
│   ├── go/                  # Go implementation (go-libp2p-rendezvous)
│   │   ├── Dockerfile
│   │   ├── cmd/node/        # Test node harness (server, registrant, discoverer)
│   │   └── ...
│   └── py/                  # Python implementation (py-libp2p)
│       ├── Dockerfile
│       ├── node.py          # Test node harness (server, registrant, discoverer)
│       └── py-libp2p/       # py-libp2p source tree
└── lib/
    ├── build-images.sh      # Image build helpers
    ├── generate-tests.sh    # Matrix generation (N³ permutations)
    └── run-single-test.sh   # Single test runner (Docker compose + Redis)
```
