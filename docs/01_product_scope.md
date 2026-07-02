# Product Scope

## Product name

Working name: **Llama Wrangler**

## One-line description

A local-first Ollama control plane that routes, parallelizes, evaluates, and observes local LLM workloads across multiple endpoints while optionally escalating unresolved deltas to frontier models.

## Problem

Local AI users often have several machines capable of running Ollama, but each machine is usually treated as an isolated endpoint. IDEs and agentic tools typically point at one model on one host, leaving other local compute underused.

Users also face a tradeoff between local privacy/cost control and frontier-model quality. Sending everything to a frontier model is expensive and potentially inappropriate. Keeping everything local can leave quality gaps.

## Solution

Llama Wrangler provides a single local API endpoint that behaves like an OpenAI-compatible and Ollama-compatible service. Behind that endpoint, a marshal node coordinates subscriber nodes running local Ollama instances.

The marshal can:

- Discover endpoint capabilities
- Track model availability and health
- Route requests to the best worker
- Fan out tasks to multiple nodes
- Compare local outputs
- Validate outputs deterministically when possible
- Escalate only unresolved deltas to frontier models when policy allows
- Emit rich observability events to Splunk through HEC

## Target users

- Local AI power users
- Software engineers using local coding models
- Homelab builders
- Security-sensitive teams wanting local-first inference
- Sales engineers and solution architects building demos
- Splunk users who want observability over local AI workloads

## Non-goals

- Do not split a single model across multiple machines.
- Do not replace Ollama.
- Do not require Kubernetes.
- Do not require cloud infrastructure.
- Do not send data to frontier models unless policy explicitly allows it.

## Success criteria

MVP is successful when:

1. A user can point an IDE at one Llama Wrangler endpoint.
2. The marshal can route requests to multiple Ollama subscribers.
3. Responses can stream back to the client.
4. Node capabilities and health are reported.
5. JSON logs and Splunk HEC events are emitted.
6. A companion Splunk app shows routing, performance, and health dashboards.
