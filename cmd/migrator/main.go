package main

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"music-platform/internal/common/config"

	_ "github.com/go-sql-driver/mysql"
)

var identPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type migrationTarget struct {
	Name    string
	Schema  string
	Keyword string
}

func main() {
	var (
		configPath    = flag.String("config", "configs/config.yaml", "配置文件路径")
		service       = flag.String("service", "all", "迁移服务: all|event|profile|media|catalog")
		migrationsDir = flag.String("dir", "migrations/sql", "迁移脚本目录")
		dryRun        = flag.Bool("dry-run", false, "只展示将执行的迁移，不实际执行")
	)
	flag.Parse()

	cfg := config.MustLoad(*configPath)
	targets, err := resolveTargets(cfg, *service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "解析迁移目标失败: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建数据库连接失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "数据库连接失败: %v\n", err)
		os.Exit(1)
	}

	for _, target := range targets {
		if err := applyTarget(db, *migrationsDir, target, *dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] 迁移失败: %v\n", target.Name, err)
			os.Exit(1)
		}
	}

	if *dryRun {
		fmt.Println("dry-run 完成")
	} else {
		fmt.Println("迁移完成")
	}
}

func resolveTargets(cfg *config.Config, service string) ([]migrationTarget, error) {
	profileSchema := normalizeSchema(cfg.Schemas.Profile, cfg.Database.Name)
	catalogSchema := normalizeSchema(cfg.Schemas.Catalog, cfg.Database.Name)
	mediaSchema := normalizeSchema(cfg.Schemas.Media, "music_media")
	eventSchema := normalizeSchema(cfg.Database.Name, "music_users")

	all := []migrationTarget{
		{Name: "event", Schema: eventSchema, Keyword: "_event_"},
		{Name: "profile", Schema: profileSchema, Keyword: "_profile_"},
		{Name: "media", Schema: mediaSchema, Keyword: "_media_"},
		{Name: "catalog", Schema: catalogSchema, Keyword: "_catalog_"},
	}

	raw := strings.TrimSpace(service)
	if raw == "" || raw == "all" {
		return all, nil
	}

	selected := make([]migrationTarget, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		found := false
		for _, t := range all {
			if t.Name == name {
				selected = append(selected, t)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("不支持的服务: %s", name)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("未选择任何服务")
	}
	return selected, nil
}

func applyTarget(db *sql.DB, migrationsDir string, target migrationTarget, dryRun bool) error {
	if target.Keyword == "" {
		return fmt.Errorf("目标缺少 keyword")
	}
	if !identPattern.MatchString(target.Schema) {
		return fmt.Errorf("非法 schema 名: %s", target.Schema)
	}

	files, err := migrationFiles(migrationsDir, target.Keyword)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Printf("[%s] 无待匹配迁移文件（keyword=%s）\n", target.Name, target.Keyword)
		return nil
	}

	if dryRun {
		fmt.Printf("[%s] 将在 schema=%s 执行 %d 个迁移:\n", target.Name, target.Schema, len(files))
		for _, f := range files {
			fmt.Printf("  - %s\n", filepath.Base(f))
		}
		return nil
	}

	if err := ensureMigrationTable(db, target.Schema); err != nil {
		return err
	}

	appliedCount := 0
	for _, file := range files {
		version := filepath.Base(file)
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("读取迁移文件失败 %s: %w", version, err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(content))

		existedChecksum, err := migrationChecksum(db, target.Schema, version)
		if err != nil {
			return err
		}
		if existedChecksum != "" {
			if existedChecksum != checksum {
				return fmt.Errorf("迁移已存在但checksum不一致: %s", version)
			}
			fmt.Printf("[%s] 跳过已执行迁移: %s\n", target.Name, version)
			continue
		}

		sqlText := strings.TrimSpace(string(content))
		if sqlText == "" {
			if err := recordMigration(db, target.Schema, version, checksum); err != nil {
				return err
			}
			continue
		}
		sqlText = fmt.Sprintf("USE %s;\n%s", quoteIdent(target.Schema), sqlText)

		if _, err := db.Exec(sqlText); err != nil {
			return fmt.Errorf("执行迁移失败 %s: %w", version, err)
		}
		if err := recordMigration(db, target.Schema, version, checksum); err != nil {
			return err
		}
		appliedCount++
		fmt.Printf("[%s] 已执行迁移: %s\n", target.Name, version)
	}

	fmt.Printf("[%s] 完成，新增执行 %d 个迁移\n", target.Name, appliedCount)
	return nil
}

func migrationFiles(dir, keyword string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		if !strings.Contains(name, keyword) {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func ensureMigrationTable(db *sql.DB, schema string) error {
	createDB := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(schema))
	if _, err := db.Exec(createDB); err != nil {
		return fmt.Errorf("创建 schema 失败 %s: %w", schema, err)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			version VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (version)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`, quoteIdent(schema), quoteIdent("schema_migrations"))
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败 %s: %w", schema, err)
	}
	return nil
}

func migrationChecksum(db *sql.DB, schema, version string) (string, error) {
	var checksum string
	query := fmt.Sprintf("SELECT checksum FROM %s.%s WHERE version = ?", quoteIdent(schema), quoteIdent("schema_migrations"))
	err := db.QueryRow(query, version).Scan(&checksum)
	if err == nil {
		return checksum, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", fmt.Errorf("查询迁移记录失败 %s: %w", version, err)
}

func recordMigration(db *sql.DB, schema, version, checksum string) error {
	query := fmt.Sprintf("INSERT INTO %s.%s (version, checksum) VALUES (?, ?)", quoteIdent(schema), quoteIdent("schema_migrations"))
	if _, err := db.Exec(query, version, checksum); err != nil {
		return fmt.Errorf("记录迁移失败 %s: %w", version, err)
	}
	return nil
}

func normalizeSchema(schema, fallback string) string {
	s := strings.TrimSpace(schema)
	if s == "" {
		s = strings.TrimSpace(fallback)
	}
	if s == "" {
		s = "music_users"
	}
	if !identPattern.MatchString(s) {
		return "music_users"
	}
	return s
}

func quoteIdent(ident string) string {
	if !identPattern.MatchString(ident) {
		return "`music_users`"
	}
	return "`" + ident + "`"
}
