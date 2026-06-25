#!/usr/bin/env python3
"""
Semantic version comparison for module releases.

Validates that a new version is greater than the current version for a given channel.
Uses the semver library for proper semantic version comparison.

Exit codes:
  0 - New version is valid (greater than current)
  1 - New version is invalid (downgrade, duplicate, or parse error)

Usage:
  check-version.py <current_base> <new_base> <channel> <current_full> <new_full>

Arguments:
  current_base - Current base version (e.g., "1.2.3")
  new_base     - New base version (e.g., "1.2.4")
  channel      - Release channel (e.g., "regular", "experimental")
  current_full - Current full version with suffix (e.g., "1.2.3-experimental")
  new_full     - New full version with suffix (e.g., "1.2.4-experimental")
"""

import sys


def main():
    """Main entry point for version comparison."""
    if len(sys.argv) != 6:
        sys.stderr.write(
            "::error::Internal error: incorrect number of arguments to version checker\n"
        )
        sys.stderr.write(f"Expected 5 arguments, got {len(sys.argv) - 1}\n")
        sys.exit(1)

    try:
        import semver
    except ImportError:
        sys.stderr.write(
            "::error::Python semver module not installed. Install with: pip install semver\n"
        )
        sys.exit(1)

    current = sys.argv[1]
    new = sys.argv[2]
    channel = sys.argv[3]
    current_version = sys.argv[4]
    version_tag = sys.argv[5]

    try:
        current_ver = semver.VersionInfo.parse(current)
        new_ver = semver.VersionInfo.parse(new)
    except Exception as e:
        sys.stderr.write(f"::error::Invalid semantic version format: {e}\n")
        sys.exit(1)

    if new_ver < current_ver:
        sys.stderr.write(f"::error::Version downgrade not allowed for {channel} channel\n")
        sys.stderr.write(f"Current version: {current_version} (base: {current})\n")
        sys.stderr.write(f"Requested version: {version_tag} (base: {new})\n")
        sys.stderr.write(f"Please use a version greater than {current}\n")
        sys.exit(1)
    elif new_ver == current_ver:
        sys.stderr.write(
            f"::error::Version {version_tag} is already released for {channel} channel\n"
        )
        sys.stderr.write(f"Current version in module-releases.yaml: {current_version}\n")
        sys.stderr.write(f"Please use a newer version number\n")
        sys.exit(1)

    # Version is valid (new_ver > current_ver)
    sys.exit(0)


if __name__ == "__main__":
    main()
