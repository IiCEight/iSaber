// owner provisions or recovers the single owner through explicit operator access.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/IiCEight/iSaber/backend/internal/database"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || (args[0] != "setup" && args[0] != "recover") {
		return errors.New("usage: owner setup|recover --username NAME [--password-stdin]")
	}
	flags := flag.NewFlagSet("owner "+args[0], flag.ContinueOnError)
	username := flags.String("username", "", "owner username")
	stdin := flags.Bool("password-stdin", false, "read one password from stdin instead of prompting")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*username) == "" {
		return errors.New("provide --username; passwords are accepted only through a hidden prompt or stdin")
	}
	if os.Getenv("OWNER_DATABASE_URL") == "" {
		return errors.New("OWNER_DATABASE_URL is required for this operator command")
	}
	password, err := readPassword(*stdin)
	if err != nil {
		return err
	}
	db, err := database.Open(os.Getenv("OWNER_DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := auth.NewStore(db)
	if args[0] == "setup" {
		err = store.Setup(ctx, *username, password)
	} else {
		err = store.Recover(ctx, *username, password)
	}
	if err != nil {
		return err
	}
	if args[0] == "setup" {
		fmt.Println("Owner created. No credentials were printed.")
	} else {
		fmt.Println("Owner credentials replaced and all prior sessions revoked.")
	}
	return nil
}

func readPassword(fromStdin bool) (string, error) {
	if fromStdin {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 4099))
		if err != nil || len(data) > 4098 {
			return "", errors.New("could not read a password within the input limit")
		}
		password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		if strings.ContainsAny(password, "\r\n") {
			return "", errors.New("stdin must contain exactly one password line")
		}
		return password, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("interactive setup requires a terminal; use --password-stdin for a secure pipe")
	}
	fmt.Fprint(os.Stderr, "New password (15–1024 characters): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", errors.New("could not read password")
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil || string(password) != string(confirmation) {
		return "", errors.New("password confirmation did not match")
	}
	return string(password), nil
}
