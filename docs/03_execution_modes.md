# Execution Modes

## single

Routes a request to the best single node based on requested model, model alias, node capability, current load, and policy.

## race

Sends a request to multiple eligible nodes and returns the first valid response. Useful for low-latency tasks where correctness can be validated quickly.

## consensus

Sends a request to multiple eligible local nodes, compares outputs, and returns the best local answer when sufficient agreement exists.

## consensus_delta

The flagship mode. Multiple local nodes participate, outputs are compared and validated, and unresolved disagreements may be escalated as a minimized Frontier Delta payload.

Recommended default for high-value small tasks, code snippets, SPL, regex, config generation, and customer-facing wording.

## committee

Assigns roles to nodes or models:

- generator
- critic
- validator
- reducer
- summarizer
- security reviewer
- code reviewer

## frontier_delta

Local-first workflow where only the unresolved delta is sent to a frontier model. The full source corpus or prompt should not be sent unless policy explicitly allows it.

## local_only

Hard block on all external provider usage.

## Recommended routing hierarchy

1. Can deterministic validation solve it?
2. Do local models agree?
3. Can a local evaluator resolve it?
4. Is frontier review allowed?
5. Send only the minimum delta.
