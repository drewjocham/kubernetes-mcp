# Google Chat Incident Bridge (CLI)
This bridge receives Google Chat events, routes incident messages (`kubernetes`/`solace`), calls a configurable investigation backend (`oz`, `llstudio`, or generic `openapi`), and posts a report with a plan of action back to the same thread.

## Run
```bash
go run ./integrations/cmd/chatbridge --config integrations/chatbridge/config.example.yaml
```
Provider-specific presets:
```bash
go run ./integrations/cmd/chatbridge --config integrations/chatbridge/config.oz.yaml
go run ./integrations/cmd/chatbridge --config integrations/chatbridge/config.oz.kube-solace.yaml
go run ./integrations/cmd/chatbridge --config integrations/chatbridge/config.llstudio.yaml
go run ./integrations/cmd/chatbridge --config integrations/chatbridge/config.openapi.yaml
```

## Config model
Use YAML to configure:
- `google_chat`: inbound auth header/token env
- `investigation`: selected provider, prompt template, timeout/polling
- `providers`: backend-specific endpoint paths, methods, output extraction paths
- `trigger`: prefixes + keyword routing + optional space allowlist
- `reliability`: idempotency TTL and retries
- `reporting`: Google Chat webhook destination

## Required environment variables (typical)
- `CHAT_BRIDGE_TOKEN` (if `auth_token_env` enabled)
- `GOOGLE_CHAT_WEBHOOK_URL`
- one provider key:
  - `OZ_API_KEY`
  - `LLSTUDIO_API_KEY`
  - `GENERIC_PROVIDER_API_KEY`

## Health
- `GET /healthz`
- `POST /chat/events`
