package atdb

import (
	"database/sql"
	"fmt"
	"reflect"
)

func PostgresConnect(uri string) (*sql.DB, error) {
	db, err := sql.Open("postgres", uri)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	fmt.Println("Successfully connected to PostgreSQL!")
	return db, nil
}

func InsertOne(db *sql.DB, query string, args ...interface{}) (int64, error) {
	var lastInsertID int64
	err := db.QueryRow(query, args...).Scan(&lastInsertID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert record: %v", err)
	}
	return lastInsertID, nil
}

func InsertMany(db *sql.DB, query string, args [][]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, arg := range args {
		_, err = stmt.Exec(arg...)
		if err != nil {
			return fmt.Errorf("failed to execute batch insert: %v", err)
		}
	}

	return tx.Commit()
}

func GetOne[T any](db *sql.DB, query string, args ...interface{}) (T, error) {
	var result T
	row := db.QueryRow(query, args...)

	// Refleksi untuk memetakan hasil query ke struct
	resultValue := reflect.ValueOf(&result).Elem()
	if resultValue.Kind() == reflect.Struct {
		// Buat slice pointer untuk setiap field dalam struct
		numFields := resultValue.NumField()
		pointers := make([]interface{}, numFields)
		for i := 0; i < numFields; i++ {
			pointers[i] = resultValue.Field(i).Addr().Interface()
		}

		// Scan hasil query ke struct
		err := row.Scan(pointers...)
		if err != nil {
			return result, fmt.Errorf("failed to fetch record: %v", err)
		}
	} else {
		// Untuk tipe non-struct
		err := row.Scan(&result)
		if err != nil {
			return result, fmt.Errorf("failed to fetch record: %v", err)
		}
	}

	return result, nil
}

func GetAll[T any](db *sql.DB, query string, args ...interface{}) ([]T, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch records: %v", err)
	}
	defer rows.Close()

	var results []T
	for rows.Next() {
		var item T
		if err := rows.Scan(&item); err != nil {
			return nil, fmt.Errorf("failed to scan record: %v", err)
		}
		results = append(results, item)
	}

	return results, nil
}

func UpdateOne(db *sql.DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to update record: %v", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows count: %v", err)
	}

	return rowsAffected, nil
}

func DeleteOne(db *sql.DB, query string, args ...interface{}) (int64, error) {
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete record: %v", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows count: %v", err)
	}

	return rowsAffected, nil
}

func GetCount(db *sql.DB, query string, args ...interface{}) (int64, error) {
	var count int64
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get count: %v", err)
	}

	return count, nil
}

func ExecuteTransaction(db *sql.DB, queries []string, args [][]interface{}) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	for i, query := range queries {
		if _, err := tx.Exec(query, args[i]...); err != nil {
			return fmt.Errorf("failed to execute query %d in transaction: %v", i+1, err)
		}
	}

	return tx.Commit()
}
