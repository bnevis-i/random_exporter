# SLSA 3 Container Build Example

## Introduction

# cmd/exporter

This is a simple prometheus metrics exporter written in golang.

The exporter generates a `bank_account_balance` floating point metric with a value between 0 and 100.

By default, the exporter listens on port 8081 and exports a /metrics endpoint.
