// SPDX-FileCopyrightText: 2020 SAP SE
// SPDX-FileCopyrightText: 2021 SAP SE
//
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/SAP/go-dblib/dsn"
)

var (
	// ASE doesn't handle creating multiple databases concurrently well.
	// To prevent spurious test errors the DBCreateLock is used to
	// synchronise the goroutines creating databases.
	DBCreateLock = new(sync.Mutex)
)

// SetupDB creates a database and sets .Database on the passed info.
func SetupDB(info interface{}) error {
	ttf := dsn.TagToField(info, dsn.OnlyJSON)
	field, ok := ttf["database"]
	if !ok {
		return fmt.Errorf("provided info does not have the 'database' field")
	}

	db, err := sql.Open("ase", dsn.FormatSimple(info))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(context.Background(), "use master"); err != nil {
		return fmt.Errorf("failed to switch context to master: %w", err)
	}

	testDatabase := "test" + RandomNumber()

	DBCreateLock.Lock()
	defer DBCreateLock.Unlock()

	if _, err := conn.ExecContext(context.Background(), fmt.Sprintf("if db_id('%s') is not null drop database %s", testDatabase, testDatabase)); err != nil {
		return fmt.Errorf("error on conditional drop of database: %w", err)
	}

	if _, err := conn.ExecContext(context.Background(), "create database "+testDatabase); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if _, err := conn.ExecContext(context.Background(), "use "+testDatabase); err != nil {
		return fmt.Errorf("failed to switch context to %s: %w", testDatabase, err)
	}

	field.SetString(testDatabase)

	return nil
}

// TeardownDB deletes the database indicated by .Database of the passed
// info and unsets the member.
func TeardownDB(info interface{}) error {
	ttf := dsn.TagToField(info, dsn.OnlyJSON)
	field, ok := ttf["database"]
	if !ok {
		return fmt.Errorf("provided info does not have the 'database' field")
	}

	db, err := sql.Open("ase", dsn.FormatSimple(info))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(context.Background(), "use master"); err != nil {
		return fmt.Errorf("failed to switch context to master: %w", err)
	}

	if _, err := conn.ExecContext(context.Background(), "drop database "+field.String()); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	field.SetString("")

	return nil
}

// SetupTableInsert creates a table with the passed type and inserts all
// passed samples as rows.
func SetupTableInsert(db *sql.DB, tableName, aseType string, samples ...interface{}) (*sql.Rows, func() error, error) {
	if _, err := db.Exec(fmt.Sprintf("create table %s (a %s)", tableName, aseType)); err != nil {
		return nil, nil, fmt.Errorf("failed to create table: %w", err)
	}

	stmt, err := db.Prepare(fmt.Sprintf("insert into %s (a) values (?)", tableName))
	if err != nil {
		return nil, nil, fmt.Errorf("error preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, sample := range samples {
		if _, err := stmt.Exec(sample); err != nil {
			return nil, nil, fmt.Errorf("failed to execute prepared statement with %v: %w", sample, err)
		}
	}

	rows, err := db.Query("select * from " + tableName)
	if err != nil {
		return nil, nil, fmt.Errorf("error selecting from %s: %w", tableName, err)
	}

	teardownFn := func() error {
		_, err := db.Exec("drop table " + tableName)
		return err
	}

	return rows, teardownFn, nil
}
