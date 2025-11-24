# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-11-24

### Added
- Initial release
- Support for YAML and JSON formats
- HTTP/HTTPS file references
- OpenAPI validation (optional)
- Circular reference detection
- Automatic format detection
- Format conversion (YAML ↔ JSON)
- Path traversal protection
- File size limits
- Recursion depth limits
- Context support for cancellation
- Functional options for configuration
- Comprehensive test coverage
- Clean architecture implementation
- Public API in `bundler` package
- CLI utility in `cmd/`

### Security
- Path traversal protection for local files
- File size limits to prevent DoS
- Recursion depth limits
- HTTP request timeouts

[Unreleased]: https://github.com/miorlan/openapi-bundler/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/miorlan/openapi-bundler/releases/tag/v0.1.0

