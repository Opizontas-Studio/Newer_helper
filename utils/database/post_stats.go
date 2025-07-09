package database

import (
	"database/sql"
	"strings"
)

func CountPostsInTimeRange(db *sql.DB, tableNames []string, startTime int64, endTime int64) (int, error) {
	if len(tableNames) == 0 {
		return 0, nil
	}

	var queryBuilder strings.Builder
	var queryArgs []interface{}

	queryBuilder.WriteString("SELECT SUM(count) FROM (")
	for i, tableName := range tableNames {
		queryBuilder.WriteString(`SELECT COUNT(*) as count FROM "`)
		queryBuilder.WriteString(tableName)
		queryBuilder.WriteString(`" WHERE timestamp >= ? AND timestamp < ?`)
		queryArgs = append(queryArgs, startTime, endTime)
		if i < len(tableNames)-1 {
			queryBuilder.WriteString(" UNION ALL ")
		}
	}
	queryBuilder.WriteString(")")

	var totalCount sql.NullInt64
	err := db.QueryRow(queryBuilder.String(), queryArgs...).Scan(&totalCount)
	if err != nil {
		return 0, err
	}
	if !totalCount.Valid {
		return 0, nil
	}

	return int(totalCount.Int64), nil
}

// GetTotalPostCount retrieves the total number of posts from all tables.
func GetTotalPostCount(db *sql.DB) (int, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var totalCount int
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return 0, err
		}

		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM "` + tableName + `"`).Scan(&count)
		if err != nil {
			return 0, err
		}
		totalCount += count
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}

	return totalCount, nil
}
