# Security Policy

## Reporting a vulnerability

HarnessLab is a research/engineering tool that can execute arbitrary code
(agent tool calls run on the host by default). Please treat security issues
seriously.

**Do not open a public issue for security vulnerabilities.** Instead, report
them privately by email to the maintainers (see the GitHub repository
profile), or use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
if enabled on the repository.

Include:

- Affected version(s) and commit(s)
- Steps to reproduce
- Impact description
- Suggested fix, if any

## Scope

- Code execution / sandbox escape in tool execution paths
- Secret leakage (API keys, tokens) in traces, bundles, or logs
- Path traversal or arbitrary file access in workspace/replay/export code
- Replay store tampering that leads to code execution

## Safe usage notes

- HarnessLab runs agent shell commands **on the host without a sandbox** by
  default. Use it only in trusted, isolated environments until sandbox
  backends are available.
- Never commit `.harness/` runtime data, replay stores, or API keys to the
  repository.
