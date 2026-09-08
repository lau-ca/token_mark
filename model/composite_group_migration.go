package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const compositeGroupNameUniqueConstraint = "uni_composite_groups_name"

// migrateCompositeGroupNameUniqueness aligns the PostgreSQL constraint name
// with GORM's name before AutoMigrate inspects the model. Older releases used
// idx_composite_groups_name or uk_composite_group_name for the same constraint.
func migrateCompositeGroupNameUniqueness(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate composite group uniqueness: database is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&CompositeGroup{}); err != nil {
		return fmt.Errorf("parse composite group schema: %w", err)
	}
	if !db.Migrator().HasTable(&CompositeGroup{}) {
		return nil
	}
	var names []string
	if err := db.Raw(`
SELECT constraint_meta.conname
FROM pg_catalog.pg_constraint AS constraint_meta
JOIN pg_catalog.pg_attribute AS attribute_meta
  ON attribute_meta.attrelid = constraint_meta.conrelid
 AND attribute_meta.attnum = constraint_meta.conkey[1]
WHERE constraint_meta.conrelid = to_regclass(?)
  AND constraint_meta.contype = 'u'
  AND cardinality(constraint_meta.conkey) = 1
  AND attribute_meta.attname = ?
ORDER BY constraint_meta.conname`, statement.Schema.Table, "name").Scan(&names).Error; err != nil {
		return fmt.Errorf("inspect composite group unique constraints: %w", err)
	}
	if len(names) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if len(names) == 1 && names[0] == compositeGroupNameUniqueConstraint {
			return nil
		}
		for _, name := range names {
			if name == compositeGroupNameUniqueConstraint {
				continue
			}
			if name != "idx_composite_groups_name" && name != "uk_composite_group_name" {
				return fmt.Errorf("composite_groups.name has unsupported unique constraint %q", name)
			}
			quoteIdentifier := func(value string) string {
				return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
			}
			query := fmt.Sprintf("ALTER TABLE %s RENAME CONSTRAINT %s TO %s", quoteIdentifier(statement.Schema.Table), quoteIdentifier(name), quoteIdentifier(compositeGroupNameUniqueConstraint))
			if err := tx.Exec(query).Error; err != nil {
				return fmt.Errorf("rename composite group unique constraint %q: %w", name, err)
			}
			break
		}
		return nil
	})
}
