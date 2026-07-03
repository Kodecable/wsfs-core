package password

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const FlagUsage = "Password source: 'stdin', 'env:NAME', or 'file:PATH'"

var ErrConflict = errors.New("URL password and --password cannot be used together")

var ErrMissingUsername = errors.New("--password requires username in URL")

func Resolve(urlPassword string, hasURLPassword bool, hasUsername bool, flagValue string, flagChanged bool) (string, error) {
	if flagChanged && !hasUsername {
		return "", ErrMissingUsername
	}
	if hasURLPassword && flagChanged {
		return "", ErrConflict
	}

	if hasURLPassword {
		return urlPassword, nil
	}
	if !flagChanged {
		return "", nil
	}

	return readSource(flagValue)
}

func readSource(source string) (string, error) {
	if source == "stdin" {
		return readStdin()
	}

	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		goto UnknownSource
	}

	switch parts[0] {
	case "env":
		return readEnv(parts[1])
	case "file":
		return readFile(parts[1])
	default:
		goto UnknownSource
	}

UnknownSource:
	return "", fmt.Errorf("unknown password source %q; want 'stdin', 'env:NAME', or 'file:PATH'", source)
}

func readStdin() (pwd string, err error) {
	fmt.Print("Password: ")

	if term.IsTerminal(int(os.Stdin.Fd())) {
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		fmt.Println("")
		pwd = string(data)
	} else {
		// Avoid read too much from stdin
		buf := make([]byte, 1)
		for {
			_, err := io.ReadFull(os.Stdin, buf)
			if err != nil {
				return "", fmt.Errorf("read password from stdin: %w", err)
			}
			if buf[0] == '\r' || buf[0] == '\n' {
				if buf[0] == '\r' {
					io.ReadFull(os.Stdin, buf) // consume '\n'
				}
				break
			}
			pwd += string(buf)
		}
		fmt.Println("")
	}

	pwd = strings.TrimRight((pwd), "\r\n")
	return
}

func readEnv(name string) (string, error) {
	if name == "" {
		return "", errors.New("password env name is empty")
	}
	value, ok := os.LookupEnv(name)
	if !ok {
		return "", fmt.Errorf("password env %q is not set", name)
	}
	return value, nil
}

func readFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("password file path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read password file %q: %w", path, err)
	}
	return strings.TrimRight((string(data)), "\r\n"), nil
}
