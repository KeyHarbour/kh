# Contributing to KeyHarbourCE

Thank you for considering contributing to **KeyHarbour kh**! We welcome bug reports, documentation improvements, and pull requests that enhance the Community Edition.

## Scope (Community vs. Enterprise)
`KeyHarbour kh` is the core, self-hosted edition of KeyHarbour. Please ensure your pull requests focus on core features, bug fixes, and performance improvements without introducing SaaS-specific dependencies.

## Getting Started Locally
1. Fork the repository and create your branch from `main` (`git checkout -b fix/my-feature`).
2. Start the development environment with Docker Compose:
   ```bash
   tbd
   ```
3. Prepare the database:
   ```bash
   tbd
   ```

## Running Tests & Linter
Before submitting a Pull Request, please make sure all tests and style checks pass:
```bash
go test -covermode=atomic -coverprofile=coverage/coverage.out ./...
go tool cover -html=coverage/coverage.out -o coverage/coverage.html
go tool cover -func=coverage/coverage.out | tee coverage/coverage.txt
```

## Pull Request Process
- Keep pull requests focused on a single bug fix or feature.
- Add or update RSpec tests for any new behavior.
- Use clear, descriptive commit messages (e.g., `feat: ...`, `fix: ...`, `docs: ...`).
- By submitting a pull request, you agree that your contributions will be licensed under the project's license.
