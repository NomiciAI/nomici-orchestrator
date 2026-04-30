## Summary

Describe the change and the user-facing outcome.

## Scope

- [ ] Code
- [ ] Docs
- [ ] Tests
- [ ] Security-sensitive behavior
- [ ] AgentSpec/schema
- [ ] Runtime/adapters

## Validation

List commands run:

```bash
make fmt
make lint
make test
make build
```

## Security notes

Call out any changes touching:

- Gateway auth or tokens
- Secrets
- Shell or filesystem access
- MCP tools
- A2A or remote agents
- OpenAI-compatible `/v1/*` endpoints
- Install or update scripts

## Checklist

- [ ] I updated docs or RFCs when behavior changed.
- [ ] I added or updated tests where appropriate.
- [ ] I did not include secrets, tokens, private traces, or private prompts.
- [ ] I considered approval and audit behavior for risky actions.
- [ ] My commits are signed off if DCO enforcement is enabled.
