// Command admin bootstraps Admin accounts (there is no self sign-up for
// Admins). It prints the generated password, the TOTP secret and the
// recovery codes once; store them safely, they are not shown again.
//
//	admin create -login ops_lead -roles SUPER_ADMIN
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	adminpkg "example.com/project-thanh/game-server/internal/admin"
	"example.com/project-thanh/game-server/internal/secretbox"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "create" {
		fmt.Fprintln(os.Stderr, "usage: admin create -login NAME -roles SUPER_ADMIN[,GAME_ADMIN,...]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	login := fs.String("login", "", "login name (a-z 0-9 _ . -, 3-32)")
	roles := fs.String("roles", "", "comma separated: SUPER_ADMIN, GAME_ADMIN, SUPPORT, FINANCE")
	_ = fs.Parse(os.Args[2:])

	var box *secretbox.Box
	if k := os.Getenv("TOTP_ENCRYPTION_KEY"); k != "" {
		b, err := secretbox.New(k)
		if err != nil {
			log.Fatal(err)
		}
		box = b
	} else if os.Getenv("THANH_ENV") == "local" {
		box = secretbox.DevBox()
	} else {
		log.Fatal("TOTP_ENCRYPTION_KEY is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	var list []string
	for _, r := range strings.Split(*roles, ",") {
		if r = strings.TrimSpace(r); r != "" {
			list = append(list, r)
		}
	}
	created, err := adminpkg.New(pool, box).Create(ctx, *login, list)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := json.MarshalIndent(created, "", "  ")
	fmt.Println(string(out))
	fmt.Fprintln(os.Stderr, "Add totp_url (or totp_secret) to an authenticator app now. This output is not shown again.")
}
