# Codex Prompt: Build Local Demo

Create a runnable local demo for Llama Wrangler.

## Demo scenario

User has:

- M4 Mac mini as marshal
- M3 Pro as MLX/general worker
- M1 Max as memory-rich worker
- i9-13900K + RTX 4090 as heavy coding worker
- Splunk Enterprise receiving HEC telemetry

## Demo requirements

1. Provide `docker-compose` or scripts where appropriate for local development.
2. Provide sample subscriber configs for:
   - m4-mini
   - m3-pro
   - m1-max
   - rtx4090
3. Provide a test client script that sends:
   - simple chat request
   - code request
   - consensus request
   - simulated failed node fallback
4. Provide sample HEC event generator for Splunk app testing.
5. Include README steps:
   - start marshal
   - start subscribers
   - configure IDE endpoint
   - view Splunk dashboards

## Deliverables

- demo configs
- test client
- sample events
- demo README
