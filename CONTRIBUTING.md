# Contributing to godex

Thanks for your interest in improving the Codex Go SDK! We welcome issues and pull requests.

## Development workflow

1. Fork and clone the repository
2. Run `go test ./...` before committing
3. Ensure `gofmt` has been applied (`gofmt -w .`)
4. For significant changes, update `README.md` and add tests

## Submitting pull requests

- Include a clear description of the change and its motivation
- Mention any relevant issues (e.g. `Fixes #123`)
- Keep commits focused; rebase onto the latest `main` when possible
- Our CI runs `go test ./...` on pushes and pull requests—make sure it passes

## Code of conduct

Be respectful and constructive. We follow the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).
