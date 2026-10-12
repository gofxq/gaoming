// Package localdeps manages only this project's native development dependencies.
package localdeps

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type Config struct {
	PG            *pgx.ConnConfig
	RedisPort     int
	RedisHost     string
	RedisPassword string
	fingerprint   string
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w; run make dev-init first", err)
	}
	var fields struct {
		DSN           string `yaml:"postgres_dsn"`
		RedisAddr     string `yaml:"redis_addr"`
		RedisPassword string `yaml:"redis_password"`
	}
	if yaml.Unmarshal(raw, &fields) != nil {
		return Config{}, fmt.Errorf("invalid YAML configuration")
	}
	uri, err := url.Parse(fields.DSN)
	if err != nil || (uri.Scheme != "postgres" && uri.Scheme != "postgresql") || uri.User == nil || uri.User.Username() == "" {
		return Config{}, fmt.Errorf("deps-up requires a postgres:// URL with explicit username and password")
	}
	if password, present := uri.User.Password(); !present || password == "" {
		return Config{}, fmt.Errorf("postgres_dsn must contain an explicit password; ambient credentials are not used for cluster initialization")
	}
	pg, err := pgx.ParseConfig(fields.DSN)
	if err != nil || fields.DSN == "" {
		return Config{}, fmt.Errorf("invalid postgres_dsn")
	}
	if !localHost(pg.Host) {
		return Config{}, fmt.Errorf("deps-up only manages a single local PostgreSQL address")
	}
	for _, fallback := range pg.Fallbacks {
		if fallback.Host != pg.Host || fallback.Port != pg.Port {
			return Config{}, fmt.Errorf("deps-up only manages a single local PostgreSQL address")
		}
	}
	if pg.User == "" || pg.Database == "" || pg.Password == "" || pg.Port == 0 {
		return Config{}, fmt.Errorf("postgres_dsn must include database, username, password and a valid port")
	}
	host, port, err := net.SplitHostPort(fields.RedisAddr)
	if err != nil || !localHost(host) {
		return Config{}, fmt.Errorf("redis_addr must be a local host:port")
	}
	redisPort, err := strconv.Atoi(port)
	if err != nil || redisPort < 1 || redisPort > 65535 {
		return Config{}, fmt.Errorf("invalid Redis port")
	}
	if fields.RedisPassword == "" {
		return Config{}, fmt.Errorf("redis_password is required for wildcard listeners; set it in %s (existing config is never overwritten)", path)
	}
	for _, secret := range []string{pg.Password, fields.RedisPassword} {
		if strings.IndexFunc(secret, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
			return Config{}, fmt.Errorf("dependency passwords must not contain non-printable characters")
		}
	}
	sum := sha256.Sum256([]byte(pg.User + "\x00" + pg.Database + "\x00" + pg.Password + "\x00" + fields.RedisPassword))
	return Config{PG: pg, RedisHost: host, RedisPort: redisPort, RedisPassword: fields.RedisPassword, fingerprint: fmt.Sprintf("%x", sum)}, nil
}

func localHost(host string) bool { return host == "localhost" || host == "127.0.0.1" || host == "::1" }

// InitConfig generates private credentials only when the destination is absent.
func InitConfig(path, template string) error {
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	raw, err := os.ReadFile(template)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("configuration template must be a YAML mapping")
	}
	password := func() (string, error) {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	pgPass, err := password()
	if err != nil {
		return err
	}
	redisPass, err := password()
	if err != nil {
		return err
	}
	mapping := doc.Content[0]
	foundPG, foundRedis := false, false
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		switch key.Value {
		case "postgres_dsn":
			u, err := url.Parse(value.Value)
			if err != nil || u.User == nil {
				return fmt.Errorf("invalid DSN template")
			}
			u.User = url.UserPassword(u.User.Username(), pgPass)
			value.Value = u.String()
			foundPG = true
		case "redis_password":
			value.Value = redisPass
			value.Tag = "!!str"
			foundRedis = true
		}
	}
	if !foundPG || !foundRedis {
		return fmt.Errorf("template lacks dependency credentials")
	}
	output, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(output); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
