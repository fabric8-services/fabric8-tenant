#!/bin/bash

. ./setenv.sh

for PACT_FILE in $(find "$PACT_DIR" -name "*.json"); do
    PACT_CONSUMER=$(jq '.["consumer"]["name"]' "$PACT_FILE" | tr -d '"')
    PACT_PROVIDER=$(jq '.["provider"]["name"]' "$PACT_FILE" | tr -d '"')

    pact-provider-verifier "$PACT_BROKER_URL/pacts/provider/$PACT_PROVIDER/consumer/$PACT_CONSUMER/versions/$PACT_VERSION" --broker-username="$PACT_BROKER_USERNAME" --broker-password="$PACT_BROKER_PASSWORD" --provider-base-url "$PACT_PROVIDER_BASE_URL"
done
