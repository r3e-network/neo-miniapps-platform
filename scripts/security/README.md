# Security Testing Scripts

This directory contains automated security testing scripts for the Neo N3 Service Layer.

## Overview

The security testing scripts help identify security vulnerabilities, dependency issues, hardcoded secrets, and API security problems in the codebase. These scripts can be run manually during development or as part of the CI/CD pipeline.

## Available Scripts

### `run_all_security_tests.sh`

Master script that runs all security tests and generates a comprehensive report.

```bash
./run_all_security_tests.sh
```

### `run_gosec.sh`

Runs the Gosec security scanner on Go code to identify security vulnerabilities.

```bash
./run_gosec.sh
```

### `scan_dependencies.sh`

Scans project dependencies for known security vulnerabilities.

```bash
./scan_dependencies.sh
```

### `detect_secrets.sh`

Detects hardcoded secrets, API keys, and credentials in the codebase.

```bash
./detect_secrets.sh
```

### `run_zap_scan.sh`

Runs OWASP ZAP to scan API endpoints for security vulnerabilities.

```bash
./run_zap_scan.sh
# or with custom target
./run_zap_scan.sh -t https://api.example.com
# or with Swagger specification
./run_zap_scan.sh -s ./path/to/openapi.yaml
```

## Prerequisites

These scripts require the following tools:

1. **Go**: Required for running Go-based security tools
2. **Docker**: Required for running OWASP ZAP
3. **Gosec**: Go security scanner (`go install github.com/securego/gosec/v2/cmd/gosec@latest`)
4. **Nancy**: Dependency vulnerability scanner (`go install github.com/sonatype-nexus-community/nancy@latest`)
5. **Gitleaks**: Secret detection tool (`go install github.com/zricethezav/gitleaks/v8@latest`)

Some of the scripts will attempt to install missing tools automatically.

## Output

All security reports are generated in the `./security-reports` directory, including:

- JSON, HTML, and text reports from Gosec
- Dependency vulnerability reports
- Secret detection reports
- ZAP API security scan reports
- A consolidated summary report

## CI/CD Integration

These scripts are integrated into the CI/CD pipeline via GitHub Actions, which runs security scans on pull requests, commits to main branches, and on a weekly schedule.

See `.github/workflows/security.yml` for the workflow configuration.

## Recommended Usage

1. Run security scans during development:
   ```bash
   ./run_all_security_tests.sh
   ```

2. Address any high-severity issues before committing code

3. Run specific scans if you only need to check certain aspects:
   ```bash
   # For Go code security
   ./run_gosec.sh
   
   # For dependency vulnerabilities
   ./scan_dependencies.sh
   ```

## Troubleshooting

If you encounter issues with the scripts:

1. Ensure all prerequisites are installed
2. Make sure scripts have execute permissions (`chmod +x *.sh`)
3. Check for errors in the log output
4. Verify that the API server is running when using `run_zap_scan.sh`

For more information on security testing, see the main [Security Testing Documentation](../../docs/security_testing.md) and [Security Automation Documentation](../../docs/security_automation.md).
