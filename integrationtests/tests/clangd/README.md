\
# Clangd Integration Tests

This directory contains integration tests for `clangd`, the C/C++ language server.

## Prerequisites

Before running these tests, you must generate the `compile_commands.json` file in the `integrationtests/workspaces/clangd` directory. This can typically be done by navigating to that directory and running a tool like `bear` with your build command (e.g., `bear -- make`).

The GitHub Actions workflow for these tests uses the following command from the root of the repository:
```bash
cd integrationtests/workspaces/clangd
bear -- make
cd ../../../..
```

## Clangd Version

These tests are currently run against **clangd version 16**. While they may pass with other versions of clangd, compatibility is not guaranteed.
