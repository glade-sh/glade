package storage

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
)

// SaveIfChanged avoids a rewrite only when Save would reproduce every row.
// The comparison is a single SQLite read transaction, so another connection's
// commit cannot produce a mixture of generations in the comparison.
func (s *SQLiteStore) SaveIfChanged(org OrgState) error {
	domain, err := NormalizeOrgDomainURL(org.DomainURL)
	if err != nil {
		return fmt.Errorf("storage: save org domainUrl: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	same, err := sqliteRowsMatch(tx, org, domain)
	if err != nil {
		return err
	}
	if same {
		return tx.Commit()
	}
	if err := tx.Rollback(); err != nil {
		return err
	}
	return s.Save(org)
}

func sqliteRowsMatch(tx *sql.Tx, org OrgState, domain string) (bool, error) {
	rows, err := tx.Query(`select key, value from org_meta`)
	if err != nil {
		return false, err
	}
	stored := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			break
		}
		stored[key] = value
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	expected := make(map[string]string, len(stored))
	for key, value := range stored {
		expected[key] = value
	}
	if err := applySQLiteOrgMetadata(expected, org, domain); err != nil {
		return false, err
	}
	for key, value := range expected {
		if value == "" {
			delete(expected, key)
		}
	}
	if !reflect.DeepEqual(stored, expected) {
		return false, nil
	}

	rows, err = tx.Query(`select name, definition_json from object_definitions`)
	if err != nil {
		return false, err
	}
	count := 0
	for rows.Next() {
		var name string
		var raw []byte
		if err = rows.Scan(&name, &raw); err != nil {
			break
		}
		object, ok := org.Objects[name]
		if !ok {
			rows.Close()
			return false, nil
		}
		var expected []byte
		expected, err = json.Marshal(object.Definition)
		if err != nil {
			break
		}
		if !bytes.Equal(raw, expected) {
			rows.Close()
			return false, nil
		}
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	if count != len(org.Objects) {
		return false, nil
	}

	rows, err = tx.Query(`select object_name, id, record_json from records`)
	if err != nil {
		return false, err
	}
	count = 0
	for rows.Next() {
		var name string
		var id ID
		var raw []byte
		if err = rows.Scan(&name, &id, &raw); err != nil {
			break
		}
		object, ok := org.Objects[name]
		if !ok {
			rows.Close()
			return false, nil
		}
		record, ok := object.Records[id]
		if !ok {
			rows.Close()
			return false, nil
		}
		var expected []byte
		expected, err = json.Marshal(record)
		if err != nil {
			break
		}
		if !bytes.Equal(raw, expected) {
			rows.Close()
			return false, nil
		}
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	expectedCount := 0
	for _, object := range org.Objects {
		expectedCount += len(object.Records)
	}
	if count != expectedCount {
		return false, nil
	}

	rows, err = tx.Query(`select object_name, sequence from id_sequences`)
	if err != nil {
		return false, err
	}
	count = 0
	for rows.Next() {
		var name string
		var sequence uint64
		if err = rows.Scan(&name, &sequence); err != nil {
			break
		}
		expected, ok := org.IDSequences[name]
		if !ok || sequence != expected {
			rows.Close()
			return false, nil
		}
		count++
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return false, err
	}
	return count == len(org.IDSequences), nil
}
