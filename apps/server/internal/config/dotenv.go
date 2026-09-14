package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, rawValue, found := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !found || !validEnvName(name) {
			return fmt.Errorf("%s:%d contains an invalid environment assignment", path, lineNumber)
		}
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		value, err := parseValue(strings.TrimSpace(rawValue))
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, lineNumber, err)
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func validEnvName(name string) bool {
	for index, character := range name {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return name != ""
}

func parseValue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if raw[0] == '"' {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", errors.New("invalid double-quoted value")
		}
		return value, nil
	}
	if raw[0] == '\'' {
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return "", errors.New("invalid single-quoted value")
		}
		return raw[1 : len(raw)-1], nil
	}
	if comment := strings.Index(raw, " #"); comment >= 0 {
		raw = raw[:comment]
	}
	return strings.TrimSpace(raw), nil
}
