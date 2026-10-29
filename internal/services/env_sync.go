package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type EnvSyncOptions struct {
	DatabaseEngine       string `json:"databaseEngine"` // "mysql", "postgres", "sqlite"
	DatabaseName         string `json:"databaseName"`
	ConfigureRedis       bool   `json:"configureRedis"`
	ConfigureMailpit     bool   `json:"configureMailpit"`
	ConfigureMeilisearch bool   `json:"configureMeilisearch"`
}

type EnvSyncResult struct {
	BackupPath string            `json:"backupPath"`
	Updated    map[string]string `json:"updated"`
}

// ApplyServicesToEnv parses and safely updates project .env with managed service credentials
func ApplyServicesToEnv(envPath string, opts EnvSyncOptions) (*EnvSyncResult, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .env file: %w", err)
	}

	// 1. Create a timestamped backup
	backupPath := fmt.Sprintf("%s.backup.%d", envPath, time.Now().Unix())
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup .env: %w", err)
	}

	content := string(data)
	updated := make(map[string]string)

	setKey := func(key, val string) {
		updated[key] = val
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^#?\s*%s\s*=.*$`, regexp.QuoteMeta(key)))
		newLine := fmt.Sprintf("%s=%s", key, val)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, newLine)
		} else {
			content = strings.TrimRight(content, "\r\n") + "\n" + newLine + "\n"
		}
	}

	// Database configuration
	switch strings.ToLower(opts.DatabaseEngine) {
	case "mysql", "mariadb":
		setKey("DB_CONNECTION", "mysql")
		setKey("DB_HOST", "127.0.0.1")
		setKey("DB_PORT", "3306")
		if opts.DatabaseName != "" {
			setKey("DB_DATABASE", opts.DatabaseName)
		}
		setKey("DB_USERNAME", "root")
		setKey("DB_PASSWORD", "")
	case "postgres", "pgsql":
		setKey("DB_CONNECTION", "pgsql")
		setKey("DB_HOST", "127.0.0.1")
		setKey("DB_PORT", "5432")
		if opts.DatabaseName != "" {
			setKey("DB_DATABASE", opts.DatabaseName)
		}
		setKey("DB_USERNAME", "postgres")
		setKey("DB_PASSWORD", "")
	case "sqlite":
		setKey("DB_CONNECTION", "sqlite")
		setKey("DB_DATABASE", "database/database.sqlite")
	}

	// Redis / Valkey
	if opts.ConfigureRedis {
		setKey("REDIS_HOST", "127.0.0.1")
		setKey("REDIS_PORT", "6379")
		setKey("REDIS_PASSWORD", "null")
		setKey("CACHE_STORE", "redis")
		setKey("QUEUE_CONNECTION", "redis")
		setKey("SESSION_DRIVER", "redis")
	}

	// Mailpit
	if opts.ConfigureMailpit {
		setKey("MAIL_MAILER", "smtp")
		setKey("MAIL_HOST", "127.0.0.1")
		setKey("MAIL_PORT", "1025")
		setKey("MAIL_USERNAME", "null")
		setKey("MAIL_PASSWORD", "null")
		setKey("MAIL_ENCRYPTION", "null")
		setKey("MAIL_FROM_ADDRESS", "hello@example.com")
	}

	// Meilisearch
	if opts.ConfigureMeilisearch {
		setKey("SCOUT_DRIVER", "meilisearch")
		setKey("MEILISEARCH_HOST", "http://127.0.0.1:7700")
		setKey("MEILISEARCH_KEY", "laradeck_master_key")
	}

	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to save updated .env: %w", err)
	}

	return &EnvSyncResult{
		BackupPath: backupPath,
		Updated:    updated,
	}, nil
}

// AutoProvisionDatabase creates the database on local MySQL or PostgreSQL if needed
func AutoProvisionDatabase(ctx context.Context, engine, dbName, phpBinary, projectPath string) error {
	cleanName := strings.ReplaceAll(dbName, "`", "")
	cleanName = strings.ReplaceAll(cleanName, ";", "")
	cleanName = strings.TrimSpace(cleanName)
	if cleanName == "" {
		return fmt.Errorf("invalid database name")
	}

	switch strings.ToLower(engine) {
	case "mysql", "mariadb":
		// Try via local mysql CLI
		if mysqlBin, err := exec.LookPath("mysql"); err == nil {
			cmd := exec.CommandContext(ctx, mysqlBin, "-h", "127.0.0.1", "-P", "3306", "-u", "root", "-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", cleanName))
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if out, err := cmd.CombinedOutput(); err == nil {
				return nil
			} else {
				_ = out
			}
		}

		// Fallback via PHP PDO
		if phpBinary != "" {
			script := fmt.Sprintf(`
try {
    $pdo = new PDO("mysql:host=127.0.0.1;port=3306", "root", "");
    $pdo->exec("CREATE DATABASE IF NOT EXISTS " . "%s" . " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;");
    echo "OK";
} catch (Exception $e) {
    echo "ERR:" . $e->getMessage();
}
`, cleanName)
			cmd := exec.CommandContext(ctx, phpBinary, "-r", script)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			out, err := cmd.CombinedOutput()
			if err == nil && strings.Contains(string(out), "OK") {
				return nil
			}
		}

	case "postgres", "pgsql":
		if phpBinary != "" {
			script := fmt.Sprintf(`
try {
    $pdo = new PDO("pgsql:host=127.0.0.1;port=5432;dbname=postgres", "postgres", "");
    $check = $pdo->query("SELECT 1 FROM pg_database WHERE datname = '%s'");
    if (!$check->fetch()) {
        $pdo->exec("CREATE DATABASE \"%s\";");
    }
    echo "OK";
} catch (Exception $e) {
    echo "ERR:" . $e->getMessage();
}
`, cleanName, cleanName)
			cmd := exec.CommandContext(ctx, phpBinary, "-r", script)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			out, err := cmd.CombinedOutput()
			if err == nil && strings.Contains(string(out), "OK") {
				return nil
			}
		}

	case "sqlite":
		if projectPath != "" {
			dbFile := filepath.Join(projectPath, "database", "database.sqlite")
			_ = os.MkdirAll(filepath.Dir(dbFile), 0755)
			if _, err := os.Stat(dbFile); os.IsNotExist(err) {
				return os.WriteFile(dbFile, []byte(""), 0644)
			}
			return nil
		}
	}

	return nil
}
