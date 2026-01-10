# Data Synchronization Process - Documentation

## Overview

This process implements a data synchronization workflow that fetches data from an external REST API, transforms and validates it, and writes it to a PostgreSQL database.

## Process Flow

1. **Fetch Data from API** - Retrieves customer data from the external REST API endpoint
2. **Transform Data** - Maps external data fields to internal schema
3. **Validate Data** - Applies quality rules and validation logic using DMN decisions
4. **Write to Database** - Persists validated data to the PostgreSQL database

## Integration Points

### REST API Connector
- **Location**: `../connectors/rest-api/`
- **Configuration**: See `resources/mappings.yaml` for endpoint details
- **Authentication**: Bearer token (configured via `API_TOKEN` environment variable)
- **Endpoints**:
  - GET `/api/v1/data` - Fetch customer records
  - PUT `/api/v1/data/{id}` - Update customer record

### Database Connector
- **Location**: `../connectors/database/`
- **Type**: PostgreSQL
- **Schema**: See `../connectors/database/schema.sql`
- **Tables**:
  - `customers` - Main customer data table
  - `sync_log` - Synchronization audit trail

## Decision Logic (DMN)

The validation decision table (`dmn/decisions.dmn.xml`) evaluates records based on:
- **Record Type**: customer, transaction, etc.
- **Quality Score**: 0-100 numeric quality metric
- **Required Fields**: Presence of mandatory fields

### Validation Outcomes
- **VALID** (Score ≥ 80) → Process immediately
- **WARNING** (Score 50-79) → Flag for manual review
- **INVALID** (Score < 50 or missing required fields) → Reject

## Case Management (CMMN)

When synchronization issues occur, a case is created to manage the investigation and resolution:

1. **Investigate Issue** - Manual task to analyze the problem
2. **Check API Status** - Automated check of API availability
3. **Check Database Status** - Automated check of database connectivity
4. **Resolve Issue** - Manual remediation task
5. **Retry Synchronization** - Automated retry of failed sync

## Configuration

### Environment Variables

Required environment variables (see `../config/secrets.example.env`):

```bash
# API Configuration
API_TOKEN=your-api-bearer-token
API_BASE_URL=https://api.example.com

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=processgit
DB_USER=sync_user
DB_PASSWORD=secure_password
```

### Data Mapping

Field mappings are defined in `resources/mappings.yaml`:

| External Field | Internal Field | Type    |
|---------------|----------------|---------|
| external_id   | customerId     | string  |
| full_name     | customerName   | string  |
| contact_email | email          | string  |
| created_date  | createdAt      | datetime|

## Error Handling

### API Failures
- **Strategy**: Retry with exponential backoff
- **Max Retries**: 3
- **Notification**: Alert ops team

### Database Failures
- **Strategy**: Log and continue
- **Notification**: Alert ops team

### Validation Failures
- **Strategy**: Reject record
- **Log Level**: Warning

## Performance Considerations

- **API Timeout**: 30 seconds
- **Database Connection Pool**: 2-10 connections
- **Batch Size**: Recommended 100 records per sync
- **Frequency**: Configurable (default: hourly)

## Testing

### Unit Tests
- Validate data transformation logic
- Test decision table rules
- Verify error handling

### Integration Tests
- End-to-end API to database flow
- Connection failure scenarios
- Data validation edge cases

### Performance Tests
- Load test with 10,000+ records
- Concurrent sync operations
- Connection pool behavior

## Monitoring

### Key Metrics
- Sync success rate
- Average processing time per record
- API response times
- Database query performance
- Error rates by type

### Alerts
- Sync failure rate > 5%
- API response time > 10s
- Database connection pool exhaustion
- Validation rejection rate > 20%

## Troubleshooting

### Common Issues

**Issue**: Sync fails with API timeout
- **Cause**: API endpoint slow or unavailable
- **Resolution**: Check API status, increase timeout, contact API provider

**Issue**: Database write failures
- **Cause**: Connection pool exhausted or schema mismatch
- **Resolution**: Check pool configuration, verify schema matches mappings

**Issue**: High validation rejection rate
- **Cause**: Data quality issues at source
- **Resolution**: Review validation rules, contact data provider

## Change Log

### Version 0.1.0 (Initial Release)
- Implemented basic data sync workflow
- Configured REST API and database connectors
- Added DMN validation rules
- Created CMMN case for issue management
- Documented configuration and operations

## Support

For issues or questions:
- **Primary Contact**: {{.RepoOwner}}@processgit.local
- **Integration Team**: integration@processgit.local
- **On-Call**: See `metadata/ownership.yaml`
