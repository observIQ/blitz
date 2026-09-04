package catalog

import (
	"fmt"
	"math/rand"
	"time"
)

// SQL Server logs to the Windows Application log under the MSSQLSERVER provider
// (the default instance; named instances use MSSQL$<INSTANCE>). It does not use
// a dedicated operational channel, so these definitions register on the
// Application channel with provider "MSSQLSERVER" (PIPE-1110).

// mssqlProvider is the event source name for the default SQL Server instance.
const mssqlProvider = "MSSQLSERVER"

// mssqlDatabases is a mix of the system databases and plausible user databases.
var mssqlDatabases = []string{
	"master", "model", "msdb", "tempdb",
	"AdventureWorks2019", "ContosoRetailDW", "SalesDB", "InventoryDB", "CustomerDB",
}

// mssqlSQLLogins are SQL-authentication login names (as opposed to Windows
// accounts), used by the SQL-auth login events.
var mssqlSQLLogins = []string{
	"sa", "app_svc", "reportuser", "webapp", "etl_user", "dbadmin",
}

// mssqlLoginFailureReasons are the real Reason strings SQL Server emits with
// event 18456.
var mssqlLoginFailureReasons = []string{
	"Password did not match that for the login provided.",
	"Could not find a login matching the name provided.",
	"Login-based server access validation failed with an infrastructure error.",
	"The account is disabled.",
	"Failed to open the explicitly specified database.",
}

func mssqlDatabase(r *rand.Rand) string {
	return mssqlDatabases[r.Intn(len(mssqlDatabases))] // #nosec G404
}

// randomLSN renders a SQL Server log sequence number in the VLF:offset:slot
// form used in backup messages (e.g. "143:3768:37").
func randomLSN(r *rand.Rand) string {
	return fmt.Sprintf("%d:%d:%d", r.Intn(900)+100, r.Intn(9000)+1000, r.Intn(120)+1) // #nosec G404
}

// mssqlBackupTimestamp renders the current time in the date(time) form SQL
// Server writes into backup messages (e.g. "01/15/2024(09:12:43)"). It tracks
// the current clock so backup events carry a live timestamp, matching how the
// record's own TimeCreated is stamped.
func mssqlBackupTimestamp() string {
	return time.Now().Format("01/02/2006(15:04:05)")
}

func init() {
	// 18453 — Login succeeded (Windows authentication).
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  18453,
		Level:    LevelInformation,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := fmt.Sprintf(`%s\%s`, opts.DomainName, PickUsername(r, opts.Usernames))
			ip := PickIP(r, opts.IPs)
			data := []EventDataField{
				{Name: "Login", Value: user},
				{Name: "AuthenticationType", Value: "Windows authentication"},
				{Name: "ClientIP", Value: ip},
			}
			return data, fmt.Sprintf("Login succeeded for user '%s'. Connection made using Windows authentication. [CLIENT: %s]", user, ip)
		},
	})

	// 18454 — Login succeeded (SQL Server authentication).
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  18454,
		Level:    LevelInformation,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := mssqlSQLLogins[r.Intn(len(mssqlSQLLogins))] // #nosec G404
			ip := PickIP(r, opts.IPs)
			data := []EventDataField{
				{Name: "Login", Value: user},
				{Name: "AuthenticationType", Value: "SQL Server authentication"},
				{Name: "ClientIP", Value: ip},
			}
			return data, fmt.Sprintf("Login succeeded for user '%s'. Connection made using SQL Server authentication. [CLIENT: %s]", user, ip)
		},
	})

	// 18456 — Login failed.
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  18456,
		Level:    LevelError,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			user := mssqlSQLLogins[r.Intn(len(mssqlSQLLogins))]                       // #nosec G404
			reason := mssqlLoginFailureReasons[r.Intn(len(mssqlLoginFailureReasons))] // #nosec G404
			ip := PickIP(r, opts.IPs)
			data := []EventDataField{
				{Name: "Login", Value: user},
				{Name: "Reason", Value: reason},
				{Name: "ClientIP", Value: ip},
			}
			return data, fmt.Sprintf("Login failed for user '%s'. Reason: %s [CLIENT: %s]", user, reason, ip)
		},
	})

	// 17137 — Starting up a database.
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  17137,
		Level:    LevelInformation,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			db := mssqlDatabase(r)
			data := []EventDataField{{Name: "Database", Value: db}}
			return data, fmt.Sprintf("Starting up database '%s'.", db)
		},
	})

	// 17187 — Server not ready to accept new client connections (startup).
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  17187,
		Level:    LevelError,
		MinRole:  RoleMember,
		Generate: func(_ *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			msg := "SQL Server is not ready to accept new client connections. Wait a few minutes before trying again. If you have access to the error log, look for the informational message that indicates that SQL Server is ready before trying to connect again."
			return []EventDataField{{Name: "Message", Value: msg}}, msg
		},
	})

	// 18264 — Database backed up.
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  18264,
		Level:    LevelInformation,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			db := mssqlDatabase(r)
			pages := r.Intn(900000) + 1000 // #nosec G404
			firstLSN, lastLSN := randomLSN(r), randomLSN(r)
			path := fmt.Sprintf(`C:\Backup\%s.bak`, db)
			data := []EventDataField{
				{Name: "Database", Value: db},
				{Name: "PagesDumped", Value: fmt.Sprintf("%d", pages)},
				{Name: "FirstLSN", Value: firstLSN},
				{Name: "LastLSN", Value: lastLSN},
				{Name: "BackupPath", Value: path},
			}
			return data, fmt.Sprintf("Database backed up. Database: %s, creation date(time): %s, pages dumped: %d, first LSN: %s, last LSN: %s, number of dump devices: 1, device information: (FILE=1, TYPE=DISK: {'%s'}). This is an informational message only. No user action is required.", db, mssqlBackupTimestamp(), pages, firstLSN, lastLSN, path)
		},
	})

	// 18265 — Log backed up.
	Register(EventDefinition{
		Channel:  "Application",
		Provider: mssqlProvider,
		EventID:  18265,
		Level:    LevelInformation,
		MinRole:  RoleMember,
		Generate: func(r *rand.Rand, _ *GenerateOpts) ([]EventDataField, string) {
			db := mssqlDatabase(r)
			firstLSN, lastLSN := randomLSN(r), randomLSN(r)
			path := fmt.Sprintf(`C:\Backup\%s.trn`, db)
			data := []EventDataField{
				{Name: "Database", Value: db},
				{Name: "FirstLSN", Value: firstLSN},
				{Name: "LastLSN", Value: lastLSN},
				{Name: "BackupPath", Value: path},
			}
			return data, fmt.Sprintf("Log was backed up. Database: %s, creation date(time): %s, first LSN: %s, last LSN: %s, number of dump devices: 1, device information: (FILE=1, TYPE=DISK: {'%s'}). This is an informational message only. No user action is required.", db, mssqlBackupTimestamp(), firstLSN, lastLSN, path)
		},
	})
}
