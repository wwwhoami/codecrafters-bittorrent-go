#!/bin/sh

# export the environment variables from .env
eval "$(sed 's/^/export /' .env)"

# If the --all flag is passed, use all test cases
if [ "$1" = "--all" ]; then
	CODECRAFTERS_TEST_CASES_JSON=$(jq -c . "${TESTER_DIR}"/test_cases.json)
else
	CODECRAFTERS_TEST_CASES_JSON=$(jq -c . "${TESTER_DIR}"/test_cases_active.json)
fi

export CODECRAFTERS_TEST_CASES_JSON

exec "${TESTER_DIR}/tester"
