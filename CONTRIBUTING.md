# Contributing

Thank you for contributing to Probelune. Discuss substantial design changes in an issue before implementation. Small bug fixes and documentation improvements can go directly to a pull request.

Use the toolchain and official package-manager commands in the README. Keep each application in its own repository. Keep modules focused; aim for handwritten source files below 300 lines. Include concise Chinese comments for non-obvious invariants and preserve English/Chinese user-facing documentation.

Explain the problem, changed behavior and validation in your pull request. Prefer a targeted regression check or a reproducible runtime scenario to large implementation-mirroring test suites. Never submit real credentials, private monitoring targets, raw customer responses, signed URLs, device keys or production databases. Synthetic fixtures must be clearly identified.

Protocol-breaking changes require an explicit migration plan and major protocol version. Do not change an existing release tag. Run the repository's formatter, static checks and build before submitting.

By submitting code you agree that your contribution is distributed under the repository license. Maintainers review changes through GitHub; acceptance and response times are not guaranteed.
