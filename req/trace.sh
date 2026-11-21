#!/usr/bin/env sh
# Checks every requirement in req/ against the code that implements it and the tests that
# verify it (docs/requirements-as-code.md). CI runs exactly this; so can you:
#
#   pip install -r req/requirements.txt
#   sh req/trace.sh
#
# The report lands in build/req/report.html. It fails on a requirement TRLC rejects, a
# tag it cannot place, a tag naming no requirement, or a requirement with no trace.
set -eu
cd "$(dirname "$0")/.."

# The requirement files are UTF-8; Python on Windows reads them as the locale's code page
# without this.
export PYTHONUTF8=1
out=build/req
mkdir -p "$out"

trlc --brief req
lobster-trlc --config req/lobster-trlc.yaml --out "$out/requirements.lobster"
go run ./cmd/lobster-tags -out "$out" internal cmd e2e web/embed.go web/src web/e2e web/signup testdata/vectors
lobster-report --lobster-config req/lobster.yaml --out "$out/report.lobster"
lobster-html-report "$out/report.lobster" --out "$out/report.html" --source-root ../..
lobster-ci-report --show-coverage "$out/report.lobster"
