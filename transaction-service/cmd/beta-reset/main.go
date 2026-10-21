// Command beta-reset deletes all player data at the end of the Closed Beta and
// keeps Admins and configuration. Read deploy/RUNBOOK_CLOSED_BETA_RESET.md
// first. Without -execute it is a dry run that only prints counts.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/project-thanh/transaction-service/internal/betareset"
)

func main() {
	execute := flag.Bool("execute", false, "really delete (default: dry run)")
	confirm := flag.String("confirm", "", `confirmation phrase: "RESET <database name>"`)
	backup := flag.String("backup-ref", "", "reference of the backup taken just before (file name, checksum, storage id)")
	operator := flag.String("operator", "", "who runs the reset (recorded in the audit log)")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())
	rep, err := betareset.Run(ctx, conn, betareset.Options{
		Env: os.Getenv("THANH_ENV"), Confirmation: *confirm, BackupRef: *backup, Operator: *operator, Execute: *execute,
	})
	if rep != nil {
		out, _ := json.MarshalIndent(rep, "", "  ")
		log.Printf("report:\n%s", out)
	}
	if err != nil {
		log.Fatal(err)
	}
	if !rep.Executed {
		log.Print("DRY RUN: nothing was deleted. Re-run with -execute -confirm \"RESET " + rep.Database + "\" -backup-ref ... -operator ...")
	}
}
