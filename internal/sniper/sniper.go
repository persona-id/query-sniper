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

	// Import the mysql driver functionality.
	_ "github.com/go-sql-driver/mysql"
	"github.com/openark/golib/sqlutils"
	"github.com/persona-id/query-sniper/internal/configuration"
)

type QuerySniper struct {
	// The name will just be the database name from the config.
	Name            string
	Connection      *sql.DB
	ReplicaLagLimit time.Duration
	HLLLimit        int
	CheckLag        bool
	QueryLimit      time.Duration
	Interval        time.Duration
	Schema          string
	LRQQuery        string
}

type MysqlProcess struct {
	ID      int            `db:"ID"`
	DB      sql.NullString `db:"DB"`
	State   sql.NullString `db:"STATE"`
	Command string         `db:"COMMAND"`
	Time    int            `db:"TIME"`
	Info    sql.NullString `db:"INFO"`
}

// Sniper constructor
//
// Parameters:
//   - name: name of the sniper, the value is the databases key from the config by default.
//   - *configuration.Config: the Viper config struct with all of the application configuration
//     including database specific values.
//
// Returns:
//   - QuerySniper: the new sniper with the correct configuration, or a blank struct on error.
//   - error: any errors that occur, or nil.
func New(name string, settings *configuration.Config) (QuerySniper, error) {
	// fetch all database specific configs via key name
	dbConfig := settings.Databases[name]
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", dbConfig.Username, dbConfig.Password, dbConfig.Address)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error opening database: %w", err)
	}

	sniper := QuerySniper{
		Name:            name,
		Connection:      db,
		ReplicaLagLimit: dbConfig.ReplicaLagLimit,
		HLLLimit:        dbConfig.HLLLimit,
		QueryLimit:      dbConfig.LongQueryLimit,
		Interval:        dbConfig.Interval,
		Schema:          dbConfig.Schema,
	}

	query, err := sniper.generateHunterQuery()
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error generating hunter query: %w", err)
	}

	sniper.LRQQuery = query

	slog.Info("Created new sniper",
		slog.String("name", sniper.Name),
		slog.String("address", dbConfig.Address),
		slog.String("schema", sniper.Schema),
		slog.String("username", dbConfig.Username),
		slog.Duration("lag_limit", sniper.ReplicaLagLimit),
		slog.Int("hll_limit", sniper.HLLLimit),
		slog.Duration("query_limit", sniper.QueryLimit),
		slog.Duration("interval", sniper.Interval),
		slog.String("hunt_query", sniper.LRQQuery),
	)

	return sniper, nil
}

// Process the configuration file and construct all required QuerySnipers from it.
//
// Parameters:
//   - context.Context: the background context for the sniper to use.
//   - *configuration.Config: settings struct, which is loaded from the config files by go-viper.
func Run(ctx context.Context, settings *configuration.Config) {
	var wg sync.WaitGroup

	for dbName := range settings.Databases {
		sniper, err := New(dbName, settings)
		if err != nil {
			slog.Error("Error in Run()", slog.Any("err", err))
			continue
		}

		wg.Add(1)

		go func(ctx context.Context, s *QuerySniper) {
			defer wg.Done()
			s.Loop(ctx)
		}(ctx, &sniper)
	}

	wg.Wait()
}

// Loop that runs in the background and checks for lag / kills queries and calls the Kill() command on queries
// that need to be removed.
//
// Parameters:
//   - context.Context: the background context for the sniper to use in the loop.
func (sniper QuerySniper) Loop(ctx context.Context) {
	ticker := time.NewTicker(sniper.Interval)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return

		case <-ticker.C:
			if sniper.ReplicaLagLimit > 0 {
				lagged, err := sniper.DetectLag()
				if err != nil {
					slog.Error("Error in sniper.DetectLag()", slog.String("instance", sniper.Name), slog.Any("err", err))
				}

				if lagged {
					slog.Warn("Lag detected, running sniper")

					queries, err := sniper.GetLongRunningQueries()
					if err != nil {
						slog.Error("Error in sniper.GetLongRunningQueries()", slog.Any("err", err))
					}

					slog.Info("LRQS", slog.Any("Queries", queries))

					sniper.KillProcesses(queries)
				} else {
					slog.Debug("No replication lag detected, sleeping",
						slog.String("instance", sniper.Name),
						slog.Duration("interval", sniper.Interval),
					)
				}
			}
		}
	}
}

// Check for either elevated history list length or replica lag. If the instance is not a replica,
// the replica lag check should return false.
// See more info at: https://lefred.be/content/a-graph-a-day-keeps-the-doctor-away-mysql-history-list-length/
//
// Returns:
//   - bool: true if either HLL or replica lag is elevated, based on the configuration values of the sniper; returns false on error.
//   - error: any errors that occur, or nil.
func (sniper QuerySniper) DetectLag() (bool, error) {
	// HLL specific
	query := "select count from information_schema.innodb_metrics where name = 'trx_rseg_history_len'"

	rows, err := sniper.Connection.Query(query)
	if err != nil {
		return false, fmt.Errorf("error querying hll count: %w", err)
	}
	defer rows.Close()

	hllCount := 0

	err = sniper.Connection.QueryRow(query).Scan(&hllCount)
	if err != nil {
		return false, fmt.Errorf("error scanning hll count: %w", err)
	}

	if hllCount > sniper.HLLLimit {
		return true, nil
	}

	// Now check for replica lag
	lag, err := sniper.GetReplicationLagFromReplicaStatus()
	if err != nil {
		return false, fmt.Errorf("error in GetReplicationLagFromReplicaStatus(): %w", err)
	}

	if lag > sniper.ReplicaLagLimit {
		return true, nil
	}

	return false, nil
}

// Get the value of replication lag, if it exists.
// This function was copied from https://github.com/github/gh-ost/ because it makes this tedious process
// much easier. Since you can't just do "select replication_lag from information_schema" or the like, this
// would required a LOT of extra code to parse the value.
//
// Returns:
//   - time.Duration: replication lag in seconds, or nil on error
//   - error: any errors that occur, or nil
func (sniper QuerySniper) GetReplicationLagFromReplicaStatus() (replicationLag time.Duration, err error) {
	err = sqlutils.QueryRowsMap(sniper.Connection, `show replica status`, func(m sqlutils.RowMap) error {
		replicaIORunning := m.GetString("Replica_IO_Running")
		replicaSQLRunning := m.GetString("Replica_SQL_Running")
		secondsBehindPrimary := m.GetNullInt64("Seconds_Behind_Source")

		if !secondsBehindPrimary.Valid {
			return fmt.Errorf("replication not running; Replica_IO_Running=%+v, Replica_SQL_Running=%+v", replicaIORunning, replicaSQLRunning)
		}

		replicationLag = time.Duration(secondsBehindPrimary.Int64) * time.Second

		return nil
	})
	if err != nil {
		return -1, fmt.Errorf("error getting replication lag: %w", err)
	}

	return replicationLag, nil
}

// Finds all long running queries, using the query generated by generateHunterQuery().
//
// Returns:
//   - []MysqlProcess: list of long running processes, or a blank list in the event of an error.
//   - error: any errors that occur, or nil.
func (sniper QuerySniper) GetLongRunningQueries() ([]MysqlProcess, error) {
	rows, err := sniper.Connection.Query(sniper.LRQQuery)
	if err != nil {
		return []MysqlProcess{}, fmt.Errorf("error getting long running queries: %w", err)
	}
	defer rows.Close()

	var processes []MysqlProcess

	for rows.Next() {
		var process MysqlProcess

		err := rows.Scan(&process.ID, &process.DB, &process.State, &process.Command, &process.Time, &process.Info)
		if err != nil {
			return []MysqlProcess{}, fmt.Errorf("error scanning long running queries: %w", err)
		}

		processes = append(processes, process)
	}

	return processes, nil
}

// Kills all of the processes in the []MysqlProcess list parameter.
//
// Parameters:
//   - []MysqlProcess, list of slow queries that need to be killed.
//
// Returns:
//   - count of killed queries.
func (sniper QuerySniper) KillProcesses(processes []MysqlProcess) int {
	killed := 0

	for _, process := range processes {
		if process.ID <= 0 {
			continue
		}

		killQuery := fmt.Sprintf("KILL %d", process.ID)

		_, err := sniper.Connection.Exec(killQuery)
		if err != nil {
			// We log here, rather than returning err, because we don't want to stop processing all of the other queries.
			slog.Error("Error killing process ID", slog.Int("process_id", process.ID), slog.Any("err", err))
			continue
		}

		killed++
	}

	return killed
}

// Generate the query used to find long running queries; uses templating to interpolate values.
//
// Returns:
//   - string: the mysql query for the sniper to use
//   - error: any errors that occur, or nil
func (sniper QuerySniper) generateHunterQuery() (string, error) {
	tmpl := template.Must(template.New("query hunter").Parse(`
		SELECT ID, DB, STATE, COMMAND, TIME, INFO
		FROM information_schema.PROCESSLIST
		WHERE COMMAND NOT IN ('Sleep', 'Killed')
		AND INFO NOT LIKE '%PROCESSLIST%'
		AND DB IS NOT NULL
		{{.TimeFilter}}
		{{.DBFilter}}
		ORDER BY TIME DESC`))

	type QueryParams struct {
		TimeFilter string
		DBFilter   string
	}

	// convert the duration into seconds for use in the query
	timeout := sniper.QueryLimit.Seconds()

	params := QueryParams{
		TimeFilter: fmt.Sprintf("AND TIME >= %d", int(timeout)),
		DBFilter:   "",
	}

	if sniper.Schema != "" {
		params.DBFilter = fmt.Sprintf("AND DB in ('%s')", sniper.Schema)
	}

	var queryBytes bytes.Buffer

	err := tmpl.Execute(&queryBytes, params)
	if err != nil {
		return "", fmt.Errorf("error executing hunter query: %w", err)
	}

	result := strings.Join(strings.Fields(queryBytes.String()), " ")

	return result, nil
}
