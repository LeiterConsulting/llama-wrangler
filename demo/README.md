# Llama Wrangler Local Demo

## Start Marshal

```bash
go run ./cmd/llama-wrangler marshal --config demo/configs/m4-mini.marshal.yaml
```

Open `http://localhost:11435/ui`.

## Start Subscribers

Run on each worker with the matching config:

```bash
go run ./cmd/llama-wrangler subscriber --config demo/configs/rtx4090.subscriber.yaml
```

## Test Client

```bash
./demo/scripts/test_client.sh
```

The script sends a simple chat request, a code-oriented request, and a consensus alias request.

## Splunk

Install `splunk_app/` into Splunk, enable HEC, configure the token in the UI, and send a sample event from the Splunk page.
