# postgres-schema-registry Specification

## Purpose

Предоставляет неизменяемый снимок доступных PostgreSQL relations и columns, чтобы Gateway проверял реальные метаданные и раскрывал только разрешённую политиками часть схемы.

## Requirements

### Requirement: Load metadata for a configured PostgreSQL datasource
The system SHALL load metadata for a named configured PostgreSQL datasource using only its configured DSN environment variable and SHALL return relations with their columns, PostgreSQL type names, and nullability without reading application rows.

#### Scenario: Load reachable datasource metadata
- **WHEN** a configured datasource is reachable and exposes a user relation with columns
- **THEN** the system returns a schema snapshot that identifies the relation and its column metadata

#### Scenario: Reject an unknown or unreachable datasource
- **WHEN** a caller requests a datasource not in validated configuration or its metadata connection fails
- **THEN** the system returns an error without returning a partial snapshot or exposing the DSN

### Requirement: Exclude system metadata from published snapshots
The system SHALL publish only user relations and SHALL exclude PostgreSQL system catalogs and `information_schema` relations from schema snapshots.

#### Scenario: Omit system catalog relations
- **WHEN** introspection runs against PostgreSQL with system schemas present
- **THEN** the resulting snapshot contains no relation from `pg_catalog` or `information_schema`

### Requirement: Validate referenced objects against one schema snapshot
The system SHALL validate a plan's datasource, qualified table, and every projection, filter, and ordering column against one immutable schema snapshot before query execution is allowed.

#### Scenario: Accept an existing referenced relation and columns
- **WHEN** every relation and column referenced by a validated plan exists in the snapshot for its datasource
- **THEN** the system reports metadata validation success without executing the plan

#### Scenario: Reject a missing table or column
- **WHEN** a validated plan references a table or column absent from the snapshot
- **THEN** the system rejects it before SQL compilation or execution

### Requirement: Return policy-filtered schema visibility
The system SHALL return for a trusted principal only configured datasource names, relations, and columns covered by that principal's select-only grants; it SHALL not reveal objects absent from those grants.

#### Scenario: Return permitted relation metadata
- **WHEN** a trusted principal has a grant for selected columns of a relation present in the snapshot
- **THEN** the visible schema includes only that relation and its granted existing columns

#### Scenario: Hide an ungranted relation
- **WHEN** a relation exists in the snapshot but is not covered by a grant for the trusted principal
- **THEN** the visible schema does not include that relation or its columns

### Requirement: Fail closed on an unavailable or inconsistent snapshot
The system SHALL refuse metadata validation and schema visibility when no usable snapshot exists for the requested datasource or when a snapshot does not match the supplied configuration boundary.

#### Scenario: Deny without a usable snapshot
- **WHEN** metadata loading has failed or the snapshot belongs to a different datasource configuration
- **THEN** the system returns a safe error and does not authorize metadata-dependent processing
