# Governance

## Project Maintainers

kiroxy is maintained by [@nopperabbo](https://github.com/nopperabbo).

## Decision Making

All decisions are made by the maintainer. Community input is welcome via GitHub Discussions and Issues.

## Contributions

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## Versioning

kiroxy follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR**: Breaking changes to the API or configuration format
- **MINOR**: New features, backward-compatible
- **PATCH**: Bug fixes, backward-compatible

## Release Process

1. Changes land on `main` via PR with passing CI
2. Maintainer tags a release (`vX.Y.Z`)
3. GitHub Actions builds, signs (cosign), generates SBOM, and publishes:
   - GitHub Release with archives + checksums
   - Container image to `ghcr.io/nopperabbo/kiroxy`
4. All release artifacts include SLSA provenance attestation

## Supply Chain Security

- All GitHub Actions are pinned to full SHA
- Releases are signed with [Sigstore](https://sigstore.dev) (keyless cosign)
- SBOM generated for every release (SPDX format)
- Dependencies scanned weekly via Dependabot + govulncheck
- OpenSSF Scorecard runs weekly
