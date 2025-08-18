# OpenTelemetry Collector Configuration - Common Package

This package provides shared configuration types and builders for OpenTelemetry Collector configurations used throughout the telemetry-manager project. Note that if a specific configuration is only used in one place, it is defined directly in that package instead of being placed here (e.g., the `filelog` receiver builder is an agent-specific log and is defined in the `logagent` package).

## Overview

The common package is organized to separate concerns clearly:
- **Type definitions** are centralized in one place for easy reference
- **Builder functions** are grouped by functionality for easy navigation
- **Constants and shared values** are consolidated for maintainability
