#!/bin/bash

set -a

# Add the current directory to Go path.
GOPATH="$GOPATH:$(pwd)"

# A directory to save pact files
PACT_DIR="${PACT_DIR:-pacts}"
PACT_CONSUMER="${PACT_CONSUMER:-Fabric8TenantService}"
PACT_PROVIDER="${PACT_PROVIDER:-Fabric8AuthService}"
