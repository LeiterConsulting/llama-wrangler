# First-Run Wizard

The embedded setup wizard is the primary configuration surface. It supports:

- local system scan
- Ollama detection
- role recommendation
- safe mDNS/Bonjour peer discovery with manual adoption
- manual subscriber enrollment by URL
- model inventory
- routing profile generation
- optional Splunk HEC setup
- Splunk HEC TLS verification toggle for trusted self-signed lab certificates
- optional Frontier provider setup placeholder
- IDE endpoint instructions
- client preset cards for Cline, Continue, Open WebUI, and generic OpenAI SDK setup
- final review and launch
- local admin token generation
- default IDE/client API key generation
- admin token rotation and local browser logout from Settings
- client API-key regenerate/revoke controls from IDE setup
- encrypted fallback secret rekey from Settings when the key source is the local key file
- client preset snippets use API-key placeholders in API responses and browser-local substitution only after a one-time key is generated
- explicit LAN exposure warning when the listen address is not localhost or loopback
- auth failure rate-limit metadata in Settings

Peer discovery is operator-initiated only. It returns review-only candidates from mDNS/Bonjour metadata and does not persist, approve, or route to discovered services. Subnet scanning remains disabled unless a future explicit opt-in workflow is added.

The wizard applies these defaults when the user selects recommended setup:

- Frontier Delta disabled
- local-only mode enabled
- metadata-only telemetry
- prompt/response payload storage disabled
- soft session affinity
- default aliases: `local-fast`, `local-code`, and `local-consensus`
- management auth required after setup completion
- client API-key auth enabled after setup completion
- LAN access disabled by default through localhost binding

Optional sections can be skipped and revisited later from Settings.
