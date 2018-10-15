#!/bin/bash

set -a

# Add the current directory to Go path.
GOPATH="$GOPATH:$(pwd)"

# A directory to save pact files
PACT_DIR="${PACT_DIR:-pacts}"
PACT_CONSUMER="${PACT_CONSUMER:-Fabric8TenantService}"
PACT_PROVIDER="${PACT_PROVIDER:-Fabric8AuthService}"

PACT_BROKER_URL="${PACT_BROKER_URL:-http://pact-broker-pact-broker.193b.starter-ca-central-1.openshiftapps.com}"
PACT_BROKER_USERNAME="${PACT_BROKER_USERNAME:-pact_broker}"
PACT_BROKER_PASSWORD="${PACT_BROKER_PASSWORD:-}"

PACT_PROVIDER_BASE_URL="${PACT_PROVIDER_BASE_URL:-https://auth.openshift.io}"
