# Validation Schemas

This directory contains validation schemas for process artifacts.

## Purpose
Validation schemas ensure that process definitions, configurations, and resources conform to organizational standards and best practices.

## Schema Types

### BPMN Schemas
- Process model validation rules
- Task naming conventions
- Gateway configuration requirements

### DMN Schemas
- Decision table structure validation
- Input/output data type validation
- Hit policy compliance

### CMMN Schemas
- Case model validation rules
- Sentry and milestone configuration
- Stage dependency validation

### Resource Schemas
- Resource mapping validation
- Binding configuration rules
- Target definition standards

## Usage
Schemas in this directory are automatically applied during:
- Process package validation
- CI/CD pipeline checks
- Pre-commit hooks
- Repository template instantiation

## Adding New Schemas
1. Create schema file in appropriate subdirectory
2. Follow JSON Schema or XML Schema standards
3. Document validation rules and examples
4. Update validation configuration to reference new schema
