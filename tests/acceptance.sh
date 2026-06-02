#!/usr/bin/env bash
set -euo pipefail

make migrate
make seed
make test
make compose-config

echo "Acceptance baseline passed. Run 'make up' for full container verification."
