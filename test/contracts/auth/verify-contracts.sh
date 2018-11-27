#!/bin/bash

. ./setenv.sh

for PACT_FILE in $(find "$PACT_DIR" -name "*.json"); do
    pact-provider-verifier "$PACT_FILE" --provider-base-url "$PACT_PROVIDER_BASE_URL"
done
