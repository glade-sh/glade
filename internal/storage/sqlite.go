package storage

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const sqliteSchemaVersion = 1

type ProjectBinding struct {
	ProjectRoot       string `json:"projectRoot,omitempty"`
	SchemaFingerprint string `json:"schemaFingerprint,omitempty"`
	SourceAPIVersion  string `json:"sourceApiVersion,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
}

const (
	metaOrgID                    = "orgId"
	metaAPIVersion               = "apiVersion"
	metaNamespace                = "namespace"
	metaOrgMetadata              = "glade.org.metadata"
	metaProjectRoot              = "glade.project.root"
	metaProjectSchemaFingerprint = "glade.project.schemaFingerprint"
	metaProjectSourceAPIVersion  = "glade.project.sourceApiVersion"
	metaProjectNamespace         = "glade.project.namespace"
)

type sqliteMigration struct {
	version    int
	statements []string
}

var sqliteMigrations = []sqliteMigration{{
	version: 1,
	statements: []string{
		`create table if not exists schema_migrations (
			version integer primary key,
			applied_at text not null default (datetime('now'))
		)`,
		`create table if not exists org_meta (
			key text primary key,
			value text not null
		)`,
		`create table if not exists object_definitions (
			name text primary key,
			definition_json blob not null
		)`,
		`create table if not exists records (
			object_name text not null,
			id text not null,
			record_json blob not null,
			primary key(object_name, id)
		)`,
		`create table if not exists id_sequences (
			object_name text primary key,
			sequence integer not null
		)`,
	},
}}

type InspectSummary struct {
	Path          string         `json:"path,omitempty"`
	SchemaVersion int            `json:"schemaVersion,omitempty"`
	Objects       int            `json:"objects"`
	Records       int            `json:"records"`
	ByObject      map[string]int `json:"byObject"`
	Users         int            `json:"users"`
	Profiles      int            `json:"profiles"`
	Permissions   int            `json:"permissions"`
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) Load() (OrgState, error) {
	org := NewOrgState()
	rows, err := s.db.Query(`select key, value from org_meta`)
	if err != nil {
		return OrgState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return OrgState{}, err
		}
		switch key {
		case metaOrgID:
			org.OrgID = value
		case metaAPIVersion:
			org.APIVersion = value
		case metaNamespace:
			org.Namespace = value
		case metaOrgMetadata:
			if err := json.Unmarshal([]byte(value), &org.Metadata); err != nil {
				return OrgState{}, fmt.Errorf("storage: decode org metadata: %w", err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return OrgState{}, err
	}
	defRows, err := s.db.Query(`select name, definition_json from object_definitions order by name`)
	if err != nil {
		return OrgState{}, err
	}
	defer defRows.Close()
	for defRows.Next() {
		var name string
		var raw []byte
		if err := defRows.Scan(&name, &raw); err != nil {
			return OrgState{}, err
		}
		var definition ObjectDefinition
		if err := json.Unmarshal(raw, &definition); err != nil {
			return OrgState{}, fmt.Errorf("storage: decode object definition %s: %w", name, err)
		}
		org.Objects[name] = ObjectState{Definition: definition, Records: make(map[ID]Record)}
	}
	if err := defRows.Err(); err != nil {
		return OrgState{}, err
	}
	recordRows, err := s.db.Query(`select object_name, id, record_json from records order by object_name, id`)
	if err != nil {
		return OrgState{}, err
	}
	defer recordRows.Close()
	for recordRows.Next() {
		var objectName string
		var id ID
		var raw []byte
		if err := recordRows.Scan(&objectName, &id, &raw); err != nil {
			return OrgState{}, err
		}
		var record Record
		if err := json.Unmarshal(raw, &record); err != nil {
			return OrgState{}, fmt.Errorf("storage: decode record %s/%s: %w", objectName, id, err)
		}
		object, ok := org.Objects[objectName]
		if !ok {
			return OrgState{}, fmt.Errorf("storage: orphan record row for undefined object %s/%s", objectName, id)
		}
		if object.Records == nil {
			object.Records = make(map[ID]Record)
		}
		object.Records[id] = record
		org.Objects[objectName] = object
	}
	if err := recordRows.Err(); err != nil {
		return OrgState{}, err
	}
	seqRows, err := s.db.Query(`select object_name, sequence from id_sequences`)
	if err != nil {
		return OrgState{}, err
	}
	defer seqRows.Close()
	for seqRows.Next() {
		var objectName string
		var sequence uint64
		if err := seqRows.Scan(&objectName, &sequence); err != nil {
			return OrgState{}, err
		}
		org.IDSequences[objectName] = sequence
	}
	if err := seqRows.Err(); err != nil {
		return OrgState{}, err
	}
	RebuildIndexes(&org)
	return org, nil
}

func (s *SQLiteStore) Save(org OrgState) error {
	meta, err := s.Metadata()
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(org.Metadata)
	if err != nil {
		return err
	}
	setMetaValue(meta, metaOrgID, org.OrgID)
	setMetaValue(meta, metaAPIVersion, org.APIVersion)
	setMetaValue(meta, metaNamespace, org.Namespace)
	setMetaValue(meta, metaOrgMetadata, string(metadata))
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`delete from org_meta`,
		`delete from object_definitions`,
		`delete from records`,
		`delete from id_sequences`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}
	metaStmt, err := tx.Prepare(`insert into org_meta(key, value) values(?, ?)`)
	if err != nil {
		return err
	}
	defer metaStmt.Close()
	for key, value := range meta {
		if value == "" {
			continue
		}
		if _, err := metaStmt.Exec(key, value); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	defStmt, err := tx.Prepare(`insert into object_definitions(name, definition_json) values(?, ?)`)
	if err != nil {
		return err
	}
	defer defStmt.Close()
	recordStmt, err := tx.Prepare(`insert into records(object_name, id, record_json) values(?, ?, ?)`)
	if err != nil {
		return err
	}
	defer recordStmt.Close()
	for _, name := range names {
		object := org.Objects[name]
		raw, err := json.Marshal(object.Definition)
		if err != nil {
			return err
		}
		if _, err := defStmt.Exec(name, raw); err != nil {
			return err
		}
		ids := make([]string, 0, len(object.Records))
		for id := range object.Records {
			ids = append(ids, string(id))
		}
		sort.Strings(ids)
		for _, idText := range ids {
			record := object.Records[ID(idText)]
			raw, err := json.Marshal(record)
			if err != nil {
				return err
			}
			if _, err := recordStmt.Exec(name, idText, raw); err != nil {
				return err
			}
		}
	}
	seqStmt, err := tx.Prepare(`insert into id_sequences(object_name, sequence) values(?, ?)`)
	if err != nil {
		return err
	}
	defer seqStmt.Close()
	for objectName, sequence := range org.IDSequences {
		if _, err := seqStmt.Exec(objectName, sequence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Metadata() (map[string]string, error) {
	rows, err := s.db.Query(`select key, value from org_meta`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	meta := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		meta[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *SQLiteStore) ProjectBinding() (ProjectBinding, bool, error) {
	meta, err := s.Metadata()
	if err != nil {
		return ProjectBinding{}, false, err
	}
	binding := ProjectBinding{
		ProjectRoot:       meta[metaProjectRoot],
		SchemaFingerprint: meta[metaProjectSchemaFingerprint],
		SourceAPIVersion:  meta[metaProjectSourceAPIVersion],
		Namespace:         meta[metaProjectNamespace],
	}
	ok := binding.ProjectRoot != "" || binding.SchemaFingerprint != "" || binding.SourceAPIVersion != "" || binding.Namespace != ""
	return binding, ok, nil
}

func (s *SQLiteStore) SetProjectBinding(binding ProjectBinding) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, key := range []string{metaProjectRoot, metaProjectSchemaFingerprint, metaProjectSourceAPIVersion, metaProjectNamespace} {
		if _, err := tx.Exec(`delete from org_meta where key = ?`, key); err != nil {
			return err
		}
	}
	values := map[string]string{
		metaProjectRoot:              binding.ProjectRoot,
		metaProjectSchemaFingerprint: binding.SchemaFingerprint,
		metaProjectSourceAPIVersion:  binding.SourceAPIVersion,
		metaProjectNamespace:         binding.Namespace,
	}
	stmt, err := tx.Prepare(`insert into org_meta(key, value) values(?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for key, value := range values {
		if value == "" {
			continue
		}
		if _, err := stmt.Exec(key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Reset(org OrgState) error {
	ResetData(&org)
	return s.Save(org)
}

func (s *SQLiteStore) Inspect(path string) (InspectSummary, error) {
	org, err := s.Load()
	if err != nil {
		return InspectSummary{}, err
	}
	summary := InspectOrg(path, org)
	version, err := s.SchemaVersion()
	if err != nil {
		return InspectSummary{}, err
	}
	summary.SchemaVersion = version
	return summary, nil
}

func InspectOrg(path string, org OrgState) InspectSummary {
	summary := InspectSummary{Path: path, Objects: len(org.Objects), ByObject: make(map[string]int, len(org.Objects))}
	for name, object := range org.Objects {
		count := len(object.Records)
		summary.ByObject[name] = count
		summary.Records += count
		switch name {
		case "User":
			summary.Users = count
		case "Profile":
			summary.Profiles = count
		case "PermissionSet", "PermissionSetAssignment":
			summary.Permissions += count
		}
	}
	return summary
}

func SchemaFingerprint(org OrgState) (string, error) {
	type fingerprintField struct {
		Name             string    `json:"name"`
		Type             FieldType `json:"type"`
		DisplayType      string    `json:"displayType,omitempty"`
		RelationshipName string    `json:"relationshipName,omitempty"`
		ReferenceTo      []string  `json:"referenceTo,omitempty"`
	}
	type fingerprintObject struct {
		Name      string             `json:"name"`
		KeyPrefix string             `json:"keyPrefix,omitempty"`
		Fields    []fingerprintField `json:"fields"`
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]fingerprintObject, 0, len(names))
	for _, name := range names {
		definition := org.Objects[name].Definition
		fieldNames := make([]string, 0, len(definition.Fields))
		for fieldName := range definition.Fields {
			fieldNames = append(fieldNames, fieldName)
		}
		sort.Strings(fieldNames)
		fields := make([]fingerprintField, 0, len(fieldNames))
		for _, fieldName := range fieldNames {
			field := definition.Fields[fieldName]
			referenceTo := append([]string(nil), field.ReferenceTo...)
			sort.Strings(referenceTo)
			fields = append(fields, fingerprintField{
				Name:             fieldName,
				Type:             field.Type,
				DisplayType:      field.DisplayType,
				RelationshipName: field.RelationshipName,
				ReferenceTo:      referenceTo,
			})
		}
		objects = append(objects, fingerprintObject{
			Name:      name,
			KeyPrefix: definition.KeyPrefix,
			Fields:    fields,
		})
	}
	payload := struct {
		Objects []fingerprintObject `json:"objects"`
	}{Objects: objects}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func setMetaValue(meta map[string]string, key, value string) {
	if value == "" {
		delete(meta, key)
		return
	}
	meta[key] = value
}

func (s *SQLiteStore) init() error {
	if _, err := s.db.Exec(`pragma foreign_keys = on`); err != nil {
		return err
	}
	for _, stmt := range []string{
		`pragma busy_timeout = 5000`,
		`pragma synchronous = normal`,
		`pragma temp_store = memory`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	version, err := s.SchemaVersion()
	if err != nil {
		return err
	}
	if version > sqliteSchemaVersion {
		return fmt.Errorf("storage: sqlite schema version %d is newer than supported version %d", version, sqliteSchemaVersion)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, migration := range sqliteMigrations {
		if migration.version <= version {
			continue
		}
		for _, stmt := range migration.statements {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("storage: apply sqlite migration %d: %w", migration.version, err)
			}
		}
		if _, err := tx.Exec(`insert or ignore into schema_migrations(version) values(?)`, migration.version); err != nil {
			return fmt.Errorf("storage: record sqlite migration %d: %w", migration.version, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("pragma user_version = %d", migration.version)); err != nil {
			return fmt.Errorf("storage: set sqlite schema version %d: %w", migration.version, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) SchemaVersion() (int, error) {
	var version int
	if err := s.db.QueryRow(`pragma user_version`).Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
