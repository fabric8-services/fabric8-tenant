#!/bin/bash

. ./setenv.sh

for PACT_FILE in $(find "$PACT_DIR" -name "*.json"); do
    PACT_CONSUMER=$(jq '.["consumer"]["name"]' "$PACT_FILE" | tr -d '"')
    PACT_PROVIDER=$(jq '.["provider"]["name"]' "$PACT_FILE" | tr -d '"')

    pact-provider-verifier "$PACT_BROKER_URL/pacts/provider/$PACT_PROVIDER/consumer/$PACT_CONSUMER/versions/$PACT_VERSION" --provider-base-url "$PACT_PROVIDER_BASE_URL"
done
