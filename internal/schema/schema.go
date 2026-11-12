// Package schema provides immutable PostgreSQL metadata snapshots.
package schema

import (
	"datawark/internal/config"
	"datawark/internal/query"
	"errors"
	"strings"
)

// Column describes one relation column as reported by PostgreSQL.
type Column struct {
	Name     string
	TypeName string
	Nullable bool
}

// Relation describes a qualified PostgreSQL relation and its columns.
type Relation struct {
	Schema  string
	Name    string
	columns []Column
}

// NewRelation constructs a relation whose column metadata cannot be modified
// through the caller's input slice.
func NewRelation(namespace, name string, columns []Column) Relation {
	return Relation{Schema: namespace, Name: name, columns: append([]Column(nil), columns...)}
}

// Columns returns a copy of this relation's ordered columns.
func (relation Relation) Columns() []Column { return append([]Column(nil), relation.columns...) }

// QualifiedName returns the grammar-compatible schema.table name.
func (relation Relation) QualifiedName() string { return relation.Schema + "." + relation.Name }

// Snapshot is immutable metadata for exactly one configured datasource.
type Snapshot struct {
	datasource string
	relations  []Relation
}

// NewSnapshot copies all supplied metadata and binds it to datasource.
func NewSnapshot(datasource string, relations []Relation) Snapshot {
	copyRelations := make([]Relation, len(relations))
	for index, relation := range relations {
		copyRelations[index] = NewRelation(relation.Schema, relation.Name, relation.columns)
	}
	return Snapshot{datasource: datasource, relations: copyRelations}
}

func (snapshot Snapshot) Datasource() string { return snapshot.datasource }

// Relations returns a copy of all relations in the snapshot.
func (snapshot Snapshot) Relations() []Relation {
	result := make([]Relation, len(snapshot.relations))
	for index, relation := range snapshot.relations {
		result[index] = NewRelation(relation.Schema, relation.Name, relation.columns)
	}
	return result
}

func (snapshot Snapshot) Relation(qualified string) (Relation, bool) {
	for _, relation := range snapshot.relations {
		if relation.QualifiedName() == qualified {
			return NewRelation(relation.Schema, relation.Name, relation.columns), true
		}
	}
	return Relation{}, false
}

func (relation Relation) Column(name string) (Column, bool) {
	for _, column := range relation.columns {
		if column.Name == name {
			return column, true
		}
	}
	return Column{}, false
}

// ValidatePlan ensures every object referenced by an already grammar-validated
// plan belongs to this exact snapshot. It never compiles or executes SQL.
func ValidatePlan(plan query.ValidatedPlan, snapshot *Snapshot) error {
	if snapshot == nil || snapshot.datasource == "" || plan.Plan.Source != snapshot.datasource {
		return errors.New("schema: metadata is unavailable")
	}
	relation, found := snapshot.Relation(plan.Plan.Table)
	if !found {
		return errors.New("schema: referenced metadata is unavailable")
	}
	for _, column := range referencedColumns(plan.Plan) {
		if _, found := relation.Column(column); !found {
			return errors.New("schema: referenced metadata is unavailable")
		}
	}
	return nil
}

// Visible derives the policy-filtered metadata that can be disclosed to a
// trusted principal. Missing or stale grants are silently omitted.
func Visible(principal string, configuration *config.Config, snapshot *Snapshot) (Snapshot, error) {
	if strings.TrimSpace(principal) == "" || configuration == nil || config.Validate(*configuration) != nil || snapshot == nil {
		return Snapshot{}, errors.New("schema: visibility is unavailable")
	}
	if _, found := config.FindDatasource(*configuration, snapshot.datasource); !found || snapshot.datasource == "" {
		return Snapshot{}, errors.New("schema: visibility is unavailable")
	}

	visible := make([]Relation, 0)
	for _, relation := range snapshot.relations {
		columns := make([]Column, 0)
		for _, policy := range configuration.Policies {
			if policy.Datasource != snapshot.datasource || !contains(policy.Principals, principal) {
				continue
			}
			for _, allow := range policy.Allow {
				if allow.Table != relation.QualifiedName() || !contains(allow.Operations, "select") {
					continue
				}
				for _, allowed := range allow.Columns {
					if column, found := relation.Column(allowed); found && !hasColumn(columns, allowed) {
						columns = append(columns, column)
					}
				}
			}
		}
		if len(columns) > 0 {
			visible = append(visible, NewRelation(relation.Schema, relation.Name, columns))
		}
	}
	return NewSnapshot(snapshot.datasource, visible), nil
}

func referencedColumns(plan query.Plan) []string {
	columns := append([]string(nil), plan.Projection...)
	for _, filter := range plan.Filters {
		columns = append(columns, filter.Column)
	}
	for _, order := range plan.Order {
		columns = append(columns, order.Column)
	}
	return columns
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func hasColumn(columns []Column, wanted string) bool {
	for _, column := range columns {
		if column.Name == wanted {
			return true
		}
	}
	return false
}
