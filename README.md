# Subs Check Custom

A Go program to test and validate proxy nodes using Xray-core. The program performs TCP and speed tests on nodes and saves the results in YAML format.

## Features

- Multiple test modes:
  - Raw nodes (no test)
  - TCP test only
  - Speed test only
  - Both TCP and speed tests
- Automatic node deduplication
- Configurable test parameters
- YAML output compatible with common proxy clients

## Usage

1. Make sure you have Xray-core installed
2. Run the program:
   ```bash
   ./subs-check-custom
   ```
3. Select a test mode:
   - 0: No test (saves all nodes to raw.yaml)
   - 1: TCP test (saves passing nodes to tcp.yaml)
   - 2: Speed test (saves passing nodes to speed.yaml)
   - 3: Both tests (saves nodes that pass both tests to best.yaml)

## Output Files

- `raw.yaml`: All nodes without testing
- `tcp.yaml`: Nodes that passed TCP test
- `speed.yaml`: Nodes that passed speed test
- `best.yaml`: Nodes that passed both TCP and speed tests

## Requirements

- Go 1.20 or later
- Xray-core
