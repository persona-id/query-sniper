package sniper

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"

	// need to import the mysql driver functionality, but we don't actually use it directly.
	_ "github.com/go-sql-driver/mysql"

	"github.com/persona-id/query-sniper/internal/configuration"
	"github.com/persona-id/query-sniper/internal/observability"
)

// QuerySniper is a struct that represents a sniper.
type QuerySniper struct {
	Connection       *sql.DB
	Name             string
	Schema           string
	LRQQuery         string
	Interval         time.Duration
	QueryLimit       time.Duration
	TransactionLimit time.Duration
	DryRun           bool
}

// MysqlProcess is a struct that represents a mysql process.
// NB: this struct is sorted by datatype to satisfy the fieldalignment linter.
type MysqlProcess struct {
	Command string         `db:"COMMAND"` // the command being executed
	DB      sql.NullString `db:"DB"`      // the database the query is running in
	User    sql.NullString `db:"USER"`    // the user executing the query
	State   sql.NullString `db:"STATE"`   // the state of the query
	Info    sql.NullString `db:"INFO"`    // the info of the query
	ID      int            `db:"ID"`      // the id of the query
	Time    int            `db:"TIME"`    // the length of time that the query has been running
}

// hunterQueryTemplate is the template for the hunter query, which is used by
// generateHunterQuery() to generate the query used to find long running queries
// for the specific sniper.
const hunterQueryTemplate = `
	SELECT ID, DB, USER,STATE, COMMAND, TIME, INFO
	FROM performance_schema.processlist
	WHERE COMMAND NOT IN ('Sleep', 'Killed')
	AND INFO NOT LIKE '%processlist%'
	AND DB IS NOT NULL
	{{if .TimeFilter}}
		{{.TimeFilter}}
	{{end}}
	{{if .DBFilter}}
		{{.DBFilter}}
	{{end}}
	ORDER BY TIME DESC`

// Run starts the sniper for each database in the settings. This is the main entry point for the sniper process, and it is
// responsible for setting up all of the snipers and then waiting for them to finish.
func Run(ctx context.Context, settings *configuration.Config) {
	var wg sync.WaitGroup

	for dbName := range settings.Databases {
		sniper, err := createSniper(dbName, settings)
		if err != nil {
			slog.Error("error in Run()",
				slog.String("db_name", dbName),
				slog.Any("err", err),
			)

			continue
		}

		// uses the new go 1.25 wg.Go() syntax
		wg.Go(func() {
			sniper.Loop(ctx)
		})
	}

	wg.Wait()
}

// Loop is the main loop for the sniper. It will find all long running queries and kill them.
func (sniper QuerySniper) Loop(ctx context.Context) {
	ticker := time.NewTicker(sniper.Interval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()

			return

		case <-ticker.C:
			queries, err := sniper.FindLongRunningQueries(ctx)
			if err != nil {
				slog.Error("error in FindLongRunningQueries()",
					slog.String("db", sniper.Name),
					slog.String("query", sniper.LRQQuery),
					slog.Any("err", err),
				)

				continue
			}

			if len(queries) > 0 {
				sniper.KillProcesses(ctx, queries)
			}
		}
	}
}

// FindLongRunningQueries finds all long running queries in the database.
func (sniper QuerySniper) FindLongRunningQueries(ctx context.Context) ([]MysqlProcess, error) {
	rows, err := sniper.Connection.QueryContext(ctx, sniper.LRQQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting long running queries: %w", err)
	}
	defer rows.Close()

	var processes []MysqlProcess

	for rows.Next() {
		var process MysqlProcess

		err = rows.Scan(&process.ID, &process.DB, &process.User, &process.State, &process.Command, &process.Time, &process.Info)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		processes = append(processes, process)
	}

	err = rows.Err()
	if err != nil {
		return []MysqlProcess{}, fmt.Errorf("error iterating over rows: %w", err)
	}

	return processes, nil
}

// KillProcesses kills the given processes.
func (sniper QuerySniper) KillProcesses(ctx context.Context, processes []MysqlProcess) int {
	killed := 0

	// TODO: add support for dry run mode here.

	for _, process := range processes {
		if process.ID <= 0 {
			continue
		}

		killQuery := fmt.Sprintf("KILL %d", process.ID)

		_, err := sniper.Connection.ExecContext(ctx, killQuery)
		if err != nil {
			// We log here, rather than returning err, because we don't want to stop processing all of the other queries.
			slog.Error("error killing mysql process",
				slog.String("db", sniper.Name),
				slog.String("user", process.User.String),
				slog.Int("process_id", process.ID),
				slog.String("state", process.State.String),
				slog.String("command", process.Command),
				slog.String("info", process.Info.String),
				slog.Any("err", err),
			)

			continue
		}

		// TODO(kuzmik): ensure that we do not log PII here. We can just use the `digest_text` field,
		// which is the digested query with params removed.
		slog.Info("killed mysql process",
			slog.String("db", sniper.Name),
			slog.String("user", process.User.String),
			slog.Int("time", process.Time),
			slog.Int("process_id", process.ID),
			slog.String("state", process.State.String),
			slog.String("command", process.Command),
			slog.String("info", process.Info.String),
		)

		// Record the metric for the killed query
		reason := "long_running_query" // default reason
		if process.Time > int(sniper.QueryLimit.Seconds()) {
			reason = "query_timeout"
		}

		observability.RecordQueryKilled(ctx, sniper.Name, reason, process.Command, process.Time)

		killed++
	}

	return killed
}

// createSniper creates a new sniper for the given database name and settings. This is NOT the entry point for the sniper library.
func createSniper(name string, settings *configuration.Config) (QuerySniper, error) {
	config := settings.Databases[name]
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", config.Username, config.Password, config.Address)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error opening database: %w", err)
	}

	sniper := QuerySniper{
		Name:             name,
		Connection:       db,
		Schema:           config.Schema,
		DryRun:           settings.DryRun,
		Interval:         config.Interval,
		QueryLimit:       config.LongQueryLimit,
		TransactionLimit: config.LongTransactionLimit,
		LRQQuery:         "",
	}

	query, err := sniper.generateHunterQuery()
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error generating hunter query: %w", err)
	}

	sniper.LRQQuery = query

	slog.Info("Created new sniper",
		slog.String("name", sniper.Name),
		slog.String("address", config.Address),
		slog.String("username", config.Username),
		slog.String("schema", sniper.Schema),
		slog.Duration("interval", sniper.Interval),
		slog.Duration("query_limit", sniper.QueryLimit),
		slog.Duration("transaction_limit", sniper.TransactionLimit),
		slog.String("hunt_query", sniper.LRQQuery),
	)

	return sniper, nil
}

// generateHunterQuery generates the query used to find long running queries for the specific sniper.
func (sniper QuerySniper) generateHunterQuery() (string, error) {
	tmpl := template.Must(
		template.New("query hunter").Parse(hunterQueryTemplate),
	)

	type QueryParams struct {
		TimeFilter string
		DBFilter   string
	}

	params := QueryParams{
		TimeFilter: fmt.Sprintf("AND TIME >= %d", int(sniper.QueryLimit.Seconds())),
	}

	if sniper.Schema != "" {
		params.DBFilter = fmt.Sprintf("AND DB in ('%s')", sniper.Schema)
	}

	var queryBytes bytes.Buffer

	err := tmpl.Execute(&queryBytes, params)
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	result := strings.Join(strings.Fields(queryBytes.String()), " ")

	return result, nil
}
