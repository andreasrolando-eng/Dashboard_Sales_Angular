package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/auth"
	"github.com/Operations-ESB/dashboard-sales/api/internal/config"
	"github.com/Operations-ESB/dashboard-sales/api/internal/db"
	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

// runUserCommand implements `server user set-password --email=EMAIL [--admin]`.
//
// It is how the very first admin gets a password (there is no one to ask in
// the UI yet), and the recovery path if every admin forgets theirs. The user
// is created if missing. The password is never taken from a flag (it would
// land in shell history and `ps`): it is asked for interactively (hidden), or
// read from stdin when piped, e.g. `echo "$PW" | server user set-password ...`.
func runUserCommand(cfg config.Config, args []string) error {
	if len(args) < 1 || args[0] != "set-password" {
		return errors.New("usage: server user set-password --email=EMAIL [--admin]")
	}
	fs := flag.NewFlagSet("user set-password", flag.ContinueOnError)
	email := fs.String("email", "", "email of the account")
	admin := fs.Bool("admin", false, "also make the account an admin")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !strings.Contains(*email, "@") {
		return errors.New("--email wajib diisi dengan email yang valid")
	}

	password, err := readPassword()
	if err != nil {
		return err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}

	gormDB, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	return setPassword(gormDB, *email, password, *admin)
}

func setPassword(gormDB *gorm.DB, email, password string, makeAdmin bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	var existing model.User
	err := gormDB.Where("email = ?", email).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if _, err := service.AddUser(gormDB, email, makeAdmin); err != nil {
			return err
		}
		fmt.Printf("user %s dibuat\n", email)
	case err != nil:
		return err
	case makeAdmin && !existing.IsAdmin:
		if err := gormDB.Model(&existing).Update("is_admin", true).Error; err != nil {
			return err
		}
		fmt.Printf("%s dijadikan admin\n", email)
	}

	svc := auth.NewService(gormDB, time.Hour)
	if err := svc.SetPassword(email, password); err != nil {
		return err
	}
	fmt.Printf("password %s diatur; semua sesi login lamanya diakhiri\n", email)
	return nil
}

func readPassword() (string, error) {
	fd := int(syscall.Stdin)
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", errors.New("password kosong: kirim lewat stdin atau jalankan di terminal")
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "Password baru: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Ulangi password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("kedua password tidak sama")
	}
	return string(first), nil
}
