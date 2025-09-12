package sniper

// We aren't doing any HTML templating here, it's solely to generate SQL queries;
// text/template is safe for our current use cases. I've disabled the semgrep rule
// for the import below.
import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"text/template" // nosemgrep: go.lang.security.audit.xss.import-text-template.import-text-template
	"time"

	// need to import the mysql driver functionality, but we don't actually use it directly.
	_ "github.com/go-sql-driver/mysql"

	"github.com/persona-id/query-sniper/internal/configuration"
)

// QuerySniper is a struct that represents a sniper.
type QuerySniper struct {
	Connection       *sql.DB
	Name             string
	Schema           string
	LRQQuery         string
	LRTXNQuery       string
	Interval         time.Duration
	QueryLimit       time.Duration
	TransactionLimit time.Duration
	DryRun           bool
}

// MysqlProcess is a struct that represents a mysql process.
// NB: this struct is sorted by datatype to satisfy the fieldalignment linter.
type MysqlProcess struct {
	Command    string         `db:"command"`        // the command being executed
	Schema     sql.NullString `db:"current_schema"` // the database the query is running in
	DigestText sql.NullString `db:"digest_text"`    // the digested query text (params removed)
	User       sql.NullString `db:"user"`           // the user executing the query
	ID         int            `db:"id"`             // the id of the query
	Time       int            `db:"time"`           // the length of time that the query has been running
}

// MysqlTransaction is a struct that represents a mysql transaction.
// NB: this struct is sorted by datatype to satisfy the fieldalignment linter.
type MysqlTransaction struct {
	Command    string         `db:"command"`        // the command being executed
	DigestText sql.NullString `db:"digest_text"`    // the digested query text (params removed)
	Query      sql.NullString `db:"query"`          // the query that the transaction is running
	Schema     sql.NullString `db:"current_schema"` // the database the transaction is running in
	State      sql.NullString `db:"trx_state"`      // the state of the transaction
	User       sql.NullString `db:"user"`           // the user executing the transaction
	ID         int            `db:"trx_id"`         // the id of the transaction
	ProcessID  int            `db:"process_id"`     // the process that the transaction is running in
	Time       int            `db:"time"`           // the length of time that the transaction has been running
}

// longQueryTemplate is the template for the long running query hunter which is used by
// generateHunterQueries() to generate the query used to find long running queries
// for the specific sniper.
const longQueryTemplate = `
	SELECT pl.id, pl.user, pl.db as current_schema, pl.command, pl.time, es.digest_text
	FROM performance_schema.processlist pl
	INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id
	INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id
	WHERE pl.command NOT IN ('sleep', 'killed')
	AND pl.info NOT LIKE '%processlist%'
	{{if .QueryTimeLimit}}
		{{.QueryTimeLimit}}
	{{end}}
	{{if .DBFilter}}
		{{.DBFilter}}
	{{end}}
	ORDER BY pl.time DESC`

// longTXNTemplate is the template for the long running transaction hunter which is used by
// generateHunterQueries() to generate the query used to find long running transactions
// for the specific sniper.
// FIXME: this might be overwrought. trx.thread_id == process_id? can we just kill the thread id?
const longTXNTemplate = `
	SELECT trx.trx_id, pl.id as process_id, trx.trx_state, TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) AS time, pl.user, pl.db as current_schema, es.digest_text
	FROM INFORMATION_SCHEMA.INNODB_TRX trx
	INNER JOIN performance_schema.processlist pl ON trx.trx_mysql_thread_id = pl.id
	INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id
	INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id
	WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= {{.TXNTimeLimit}}
	{{if .DBFilter}}
		{{.DBFilter}}
	{{end}}
	ORDER BY time DESC`

// Run starts the sniper for each database in the settings. This is the main entry
// point for the sniper process, and it is responsible for setting up all snipers
// and then waiting for them to finish.
func Run(ctx context.Context, settings *configuration.Config) {
	var wg sync.WaitGroup

	for dbName := range settings.Databases {
		sniper, err := New(dbName, settings)
		if err != nil {
			slog.Error("Error in Run()",
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

// New creates a new sniper for the given database name and settings.
// This is NOT the entry point for the sniper library.
func New(name string, settings *configuration.Config) (QuerySniper, error) {
	config := settings.Databases[name]
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", config.Username, config.Password, config.Address, config.Port)

	// Configure SSL/TLS based on provided certificates:
	// - CA-only mode: Just ssl_ca for encrypted connections without client auth
	// - Mutual TLS mode: All three (ssl_ca, ssl_cert, ssl_key) for mutual authentication
	// - Invalid partial combinations (cert without CA, etc.) are ignored
	if config.SSLCA != "" && config.SSLCert == "" && config.SSLKey == "" {
		// CA-only mode: Basic encrypted connection
		dsn += "?tls=true&tls_ca=" + config.SSLCA
	} else if config.SSLCert != "" && config.SSLKey != "" && config.SSLCA != "" {
		// Mutual TLS mode: Full mutual authentication
		dsn += fmt.Sprintf("?tls=true&tls_cert=%s&tls_key=%s&tls_ca=%s", config.SSLCert, config.SSLKey, config.SSLCA)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error opening database: %w", err)
	}

	// Global safe-mode overrides any per-database dry_run setting
	// In other words, if settings.SafeMode is true, and a
	// given sniper.Config.DryRun is set to false,
	// the sniper will log and NOT kill queries.
	dryRun := config.DryRun || settings.SafeMode

	sniper := QuerySniper{
		Connection:       db,
		DryRun:           dryRun,
		Interval:         config.Interval,
		LRQQuery:         "",
		LRTXNQuery:       "",
		Name:             name,
		QueryLimit:       config.LongQueryLimit,
		Schema:           config.Schema,
		TransactionLimit: config.LongTransactionLimit,
	}

	query, txn, err := sniper.hunterQueries()
	if err != nil {
		return QuerySniper{}, fmt.Errorf("error generating hunter queries: %w", err)
	}

	sniper.LRQQuery = query
	sniper.LRTXNQuery = txn

	slog.Info("Created new sniper",
		slog.String("name", sniper.Name),
		slog.String("address", config.Address),
		slog.Int("port", config.Port),
		slog.String("username", config.Username),
		slog.String("schema", sniper.Schema),
		slog.Duration("interval", sniper.Interval),
		slog.Duration("query_limit", sniper.QueryLimit),
		slog.Duration("transaction_limit", sniper.TransactionLimit),
		slog.Bool("dry_run", sniper.DryRun),
		slog.Bool("safe_mode_active", settings.SafeMode),
		slog.String("lrq_query", sniper.LRQQuery),
		slog.String("lrtxn_query", sniper.LRTXNQuery),
	)

	return sniper, nil
}

// Loop is the main loop for the sniper. It will find all long running queries and kill them.
func (sniper QuerySniper) Loop(ctx context.Context) {
	ticker := time.NewTicker(sniper.Interval)

	for {
		select {
		case <-ctx.Done():
			slog.Debug("Context done, stopping ticker", slog.String("db", sniper.Name))

			ticker.Stop()

			return

		case <-ticker.C:
			queries, err := sniper.FindLongRunningQueries(ctx)
			if err != nil {
				slog.Error("Error in FindLongRunningQueries()",
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

		err = rows.Scan(&process.ID, &process.User, &process.Schema, &process.Command, &process.Time, &process.DigestText)
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

// FindLongRunningTransactions finds long running transactions based on the configured transaction limit.
func (sniper QuerySniper) FindLongRunningTransactions(ctx context.Context) ([]MysqlTransaction, error) {
	rows, err := sniper.Connection.QueryContext(ctx, sniper.LRTXNQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting long running transactions: %w", err)
	}
	defer rows.Close()

	var transactions []MysqlTransaction

	for rows.Next() {
		var transaction MysqlTransaction

		err = rows.Scan(&transaction.ID, &transaction.ProcessID, &transaction.State, &transaction.Time, &transaction.User, &transaction.Schema, &transaction.DigestText)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}

		transactions = append(transactions, transaction)
	}

	err = rows.Err()
	if err != nil {
		return []MysqlTransaction{}, fmt.Errorf("error iterating over rows: %w", err)
	}

	return transactions, nil
}

// KillProcesses kills the given processes, or logs them if in dry run mode.
func (sniper QuerySniper) KillProcesses(ctx context.Context, processes []MysqlProcess) int {
	killed := 0

	for _, process := range processes {
		if process.ID <= 0 {
			// im not entirely sure how this would happen
			continue
		}

		if sniper.DryRun {
			// In dry run mode, only log what would be killed
			slog.Info("DRY RUN - Would kill mysql process",
				slog.String("db", sniper.Name),
				slog.String("user", process.User.String),
				slog.Bool("dry_run", sniper.DryRun),
				slog.Int("time", process.Time),
				slog.Int("process_id", process.ID),
				slog.String("command", process.Command),
				slog.String("schema", process.Schema.String),
				slog.String("digest_text", process.DigestText.String),
			)

			killed++

			continue
		}

		killQuery := fmt.Sprintf("KILL %d", process.ID)

		_, err := sniper.Connection.ExecContext(ctx, killQuery)
		if err != nil {
			// we log here, rather than returning err, because we don't want to stop processing all of the other queries.
			slog.Error("Error killing mysql process",
				slog.String("db", sniper.Name),
				slog.String("user", process.User.String),
				slog.Bool("dry_run", sniper.DryRun),
				slog.Int("time", process.Time),
				slog.Int("process_id", process.ID),
				slog.String("command", process.Command),
				slog.String("schema", process.Schema.String),
				slog.String("digest_text", process.DigestText.String),
				slog.Any("err", err),
			)

			continue
		}

		// Using digest_text instead of raw query info to avoid logging PII
		slog.Info("Killed mysql process",
			slog.String("db", sniper.Name),
			slog.String("user", process.User.String),
			slog.Bool("dry_run", sniper.DryRun),
			slog.Int("time", process.Time),
			slog.Int("process_id", process.ID),
			slog.String("command", process.Command),
			slog.String("schema", process.Schema.String),
			slog.String("digest_text", process.DigestText.String),
		)

		killed++
	}

	return killed
}

// hunterQueries generates the query used to find long running queries
// for the specific sniper.
func (sniper QuerySniper) hunterQueries() (string, string, error) {
	// compile the query to detect long running queries
	lrqTmpl := template.Must(
		template.New("query hunter").Parse(longQueryTemplate),
	)

	type QueryParams struct {
		DBFilter       string
		QueryTimeLimit string
		TXNTimeLimit   string
	}

	params := QueryParams{
		QueryTimeLimit: fmt.Sprintf("AND pl.time >= %d", int(sniper.QueryLimit.Seconds())),
		TXNTimeLimit:   strconv.Itoa(int(sniper.TransactionLimit.Seconds())),
	}

	if sniper.Schema != "" {
		params.DBFilter = fmt.Sprintf("AND pl.db IN ('%s')", sniper.Schema)
	}

	var queryBytes bytes.Buffer

	err := lrqTmpl.Execute(&queryBytes, params)
	if err != nil {
		return "", "", fmt.Errorf("error executing template: %w", err)
	}

	query := strings.Join(strings.Fields(queryBytes.String()), " ")

	// compile the query to detect long running transactions
	txnTmpl := template.Must(
		template.New("query hunter").Parse(longTXNTemplate),
	)

	var txnBytes bytes.Buffer

	err = txnTmpl.Execute(&txnBytes, params)
	if err != nil {
		return "", "", fmt.Errorf("error executing template: %w", err)
	}

	txn := strings.Join(strings.Fields(txnBytes.String()), " ")

	return query, txn, nil
}
