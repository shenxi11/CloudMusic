package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"music-platform/internal/common/config"

	_ "github.com/go-sql-driver/mysql"
)

var identPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

var audioExt = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".ogg":  {},
	".wav":  {},
	".m4a":  {},
	".aac":  {},
	".dsf":  {},
	".ape":  {},
}

var videoExt = map[string]struct{}{
	".mp4":  {},
	".mkv":  {},
	".mov":  {},
	".avi":  {},
	".webm": {},
	".flv":  {},
	".m4v":  {},
	".ts":   {},
}

type mediaFile struct {
	FullPath string
	RelPath  string
	Title    string
	Artist   string
	Album    string
	Duration float64
	Size     int64
	FileType string
	IsAudio  bool
	LrcPath  *string
	Cover    *string
}

type stats struct {
	AudioScanned       int
	VideoScanned       int
	MusicUpserted      int
	ArtistsUpserted    int
	LyricsMapUpserted  int
	LyricsMapDeleted   int
	SkippedTooLongPath int
	Warnings           int
}

func main() {
	var (
		configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
		audioDir   = flag.String("audio-dir", "", "音频目录（默认使用 server.upload_dir）")
		videoDir   = flag.String("video-dir", "", "视频目录（默认使用 server.video_dir）")
		skipVideo  = flag.Bool("skip-video", false, "跳过视频目录扫描")
		dryRun     = flag.Bool("dry-run", false, "仅扫描并输出，不写数据库")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fatalf("加载配置失败: %v", err)
	}

	baseDir := filepath.Dir(*configPath)
	uploadRoot := ""
	if strings.TrimSpace(*audioDir) != "" {
		uploadRoot = resolvePath(".", strings.TrimSpace(*audioDir))
	} else {
		uploadRoot = resolvePath(baseDir, strings.TrimSpace(cfg.Server.UploadDir))
	}
	videoRoot := ""
	if strings.TrimSpace(*videoDir) != "" {
		videoRoot = resolvePath(".", strings.TrimSpace(*videoDir))
	} else {
		videoRoot = resolvePath(baseDir, strings.TrimSpace(cfg.Server.VideoDir))
	}

	if uploadRoot == "" {
		fatalf("音频目录为空，请通过 --audio-dir 指定")
	}
	if !*skipVideo && videoRoot == "" {
		fatalf("视频目录为空，请通过 --video-dir 指定，或使用 --skip-video")
	}

	catalogSchema := strings.TrimSpace(cfg.Schemas.Catalog)
	if catalogSchema == "" {
		catalogSchema = strings.TrimSpace(cfg.Database.Name)
	}
	mediaSchema := strings.TrimSpace(cfg.Schemas.Media)
	if mediaSchema == "" {
		mediaSchema = "music_media"
	}
	validateIdentOrFail(catalogSchema, "catalog schema")
	validateIdentOrFail(mediaSchema, "media schema")

	fmt.Printf("配置文件: %s\n", *configPath)
	fmt.Printf("音频目录: %s\n", uploadRoot)
	if *skipVideo {
		fmt.Printf("视频目录: (已跳过)\n")
	} else {
		fmt.Printf("视频目录: %s\n", videoRoot)
	}
	if *dryRun {
		fmt.Println("模式: dry-run（不写数据库）")
	}

	hasFFProbe := commandExists("ffprobe")
	if !hasFFProbe {
		fmt.Println("提示: 未检测到 ffprobe，duration/title/artist/album 将使用文件名和默认值")
	}

	audioFiles, stAudio := scanAudio(uploadRoot, hasFFProbe)
	videoFiles, stVideo := []mediaFile{}, stats{}
	if !*skipVideo {
		videoFiles, stVideo = scanVideo(videoRoot, hasFFProbe)
	}

	st := stats{
		AudioScanned:       stAudio.AudioScanned,
		VideoScanned:       stVideo.VideoScanned,
		SkippedTooLongPath: stAudio.SkippedTooLongPath + stVideo.SkippedTooLongPath,
		Warnings:           stAudio.Warnings + stVideo.Warnings,
	}

	fmt.Printf("扫描完成: 音频=%d, 视频=%d\n", len(audioFiles), len(videoFiles))
	if *dryRun {
		printStats(st)
		return
	}

	db, err := openDB(&cfg.Database)
	if err != nil {
		fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	if err := ensureMediaLyricsTable(db, mediaSchema); err != nil {
		fatalf("初始化 media_lyrics_map 失败: %v", err)
	}

	applyStats, err := upsertAll(db, catalogSchema, mediaSchema, audioFiles, videoFiles)
	if err != nil {
		fatalf("写入数据库失败: %v", err)
	}

	st.MusicUpserted = applyStats.MusicUpserted
	st.ArtistsUpserted = applyStats.ArtistsUpserted
	st.LyricsMapUpserted = applyStats.LyricsMapUpserted
	st.LyricsMapDeleted = applyStats.LyricsMapDeleted

	printStats(st)
}

func openDB(cfg *config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func scanAudio(root string, hasFFProbe bool) ([]mediaFile, stats) {
	files := make([]mediaFile, 0, 256)
	st := stats{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			st.Warnings++
			fmt.Printf("警告: 无法访问 %s: %v\n", path, walkErr)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if _, ok := audioExt[ext]; !ok {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			st.Warnings++
			fmt.Printf("警告: 计算相对路径失败 %s: %v\n", path, err)
			return nil
		}
		rel = filepath.ToSlash(rel)
		if len(rel) > 255 {
			st.SkippedTooLongPath++
			fmt.Printf("警告: 路径超过 255，跳过: %s\n", rel)
			return nil
		}

		baseNoExt := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		parentDir := filepath.Base(filepath.Dir(path))
		title, artist := inferTitleArtist(baseNoExt, parentDir)
		album := ""
		duration := 0.0

		if hasFFProbe {
			if md, err := probeWithFFProbe(path); err == nil {
				if strings.TrimSpace(md.Title) != "" {
					title = md.Title
				}
				if strings.TrimSpace(md.Artist) != "" {
					artist = md.Artist
				}
				if strings.TrimSpace(md.Album) != "" {
					album = md.Album
				}
				if md.Duration > 0 {
					duration = md.Duration
				}
			}
		}

		lrc := detectLRC(root, path)
		cover := detectCover(root, path)

		files = append(files, mediaFile{
			FullPath: path,
			RelPath:  rel,
			Title:    truncate(title, 255),
			Artist:   truncate(artist, 255),
			Album:    truncate(album, 255),
			Duration: duration,
			Size:     info.Size(),
			FileType: ext,
			IsAudio:  true,
			LrcPath:  lrc,
			Cover:    cover,
		})
		st.AudioScanned++
		return nil
	})

	if err != nil {
		st.Warnings++
		fmt.Printf("警告: 扫描音频目录失败: %v\n", err)
	}
	return files, st
}

func scanVideo(root string, hasFFProbe bool) ([]mediaFile, stats) {
	files := make([]mediaFile, 0, 128)
	st := stats{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			st.Warnings++
			fmt.Printf("警告: 无法访问 %s: %v\n", path, walkErr)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if _, ok := videoExt[ext]; !ok {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			st.Warnings++
			fmt.Printf("警告: 计算相对路径失败 %s: %v\n", path, err)
			return nil
		}
		rel = filepath.ToSlash(rel)
		if len(rel) > 255 {
			st.SkippedTooLongPath++
			fmt.Printf("警告: 路径超过 255，跳过: %s\n", rel)
			return nil
		}

		title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		duration := 0.0
		if hasFFProbe {
			if md, err := probeWithFFProbe(path); err == nil {
				if strings.TrimSpace(md.Title) != "" {
					title = md.Title
				}
				if md.Duration > 0 {
					duration = md.Duration
				}
			}
		}

		files = append(files, mediaFile{
			FullPath: path,
			RelPath:  rel,
			Title:    truncate(title, 255),
			Artist:   "",
			Album:    "",
			Duration: duration,
			Size:     info.Size(),
			FileType: ext,
			IsAudio:  false,
		})
		st.VideoScanned++
		return nil
	})

	if err != nil {
		st.Warnings++
		fmt.Printf("警告: 扫描视频目录失败: %v\n", err)
	}
	return files, st
}

type ffprobeMeta struct {
	Duration float64
	Title    string
	Artist   string
	Album    string
}

func probeWithFFProbe(path string) (*ffprobeMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration:format_tags=title,artist,album",
		"-of", "default=noprint_wrappers=1:nokey=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	meta := &ffprobeMeta{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "duration":
			if f, e := strconv.ParseFloat(val, 64); e == nil && f >= 0 {
				meta.Duration = f
			}
		case "TAG:title":
			meta.Title = val
		case "TAG:artist":
			meta.Artist = val
		case "TAG:album":
			meta.Album = val
		}
	}
	return meta, nil
}

func inferTitleArtist(baseName string, parentDir string) (string, string) {
	title := strings.TrimSpace(baseName)
	artist := ""

	if left, right, ok := splitArtistTitle(baseName); ok {
		artist = strings.TrimSpace(left)
		title = strings.TrimSpace(right)
		return title, artist
	}

	if left, _, ok := splitArtistTitle(parentDir); ok {
		artist = strings.TrimSpace(left)
	}

	return title, artist
}

func splitArtistTitle(s string) (string, string, bool) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return "", "", false
	}
	if strings.Contains(raw, " - ") {
		parts := strings.SplitN(raw, " - ", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			return parts[0], parts[1], true
		}
	}
	if strings.Count(raw, "-") == 1 {
		parts := strings.SplitN(raw, "-", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			return parts[0], parts[1], true
		}
	}
	return "", "", false
}

func detectLRC(root string, audioPath string) *string {
	baseNoExt := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))
	candidate := baseNoExt + ".lrc"
	if fileExists(candidate) {
		rel, err := filepath.Rel(root, candidate)
		if err == nil {
			s := filepath.ToSlash(rel)
			if len(s) <= 255 {
				return &s
			}
		}
	}

	dir := filepath.Dir(audioPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	lrcFiles := make([]string, 0, 2)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".lrc") {
			lrcFiles = append(lrcFiles, filepath.Join(dir, e.Name()))
		}
	}
	if len(lrcFiles) == 1 {
		rel, err := filepath.Rel(root, lrcFiles[0])
		if err == nil {
			s := filepath.ToSlash(rel)
			if len(s) <= 255 {
				return &s
			}
		}
	}
	return nil
}

func detectCover(root string, audioPath string) *string {
	exts := []string{".png", ".jpg", ".jpeg", ".webp"}
	baseNoExt := strings.TrimSuffix(audioPath, filepath.Ext(audioPath))

	candidates := make([]string, 0, 12)
	for _, ext := range exts {
		candidates = append(candidates,
			baseNoExt+"_cover"+ext,
			baseNoExt+ext,
			filepath.Join(filepath.Dir(audioPath), "cover"+ext),
		)
	}

	for _, c := range candidates {
		if !fileExists(c) {
			continue
		}
		rel, err := filepath.Rel(root, c)
		if err != nil {
			continue
		}
		s := filepath.ToSlash(rel)
		if len(s) <= 255 {
			return &s
		}
	}
	return nil
}

func upsertAll(db *sql.DB, catalogSchema, mediaSchema string, audioFiles, videoFiles []mediaFile) (stats, error) {
	all := make([]mediaFile, 0, len(audioFiles)+len(videoFiles))
	all = append(all, audioFiles...)
	all = append(all, videoFiles...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].IsAudio != all[j].IsAudio {
			return all[i].IsAudio
		}
		return all[i].RelPath < all[j].RelPath
	})

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return stats{}, err
	}
	defer func() { _ = tx.Rollback() }()

	musicTable := qualifiedName(catalogSchema, "music_files")
	artistTable := qualifiedName(catalogSchema, "artists")
	lyricsTable := qualifiedName(mediaSchema, "media_lyrics_map")

	upsertMusicSQL := fmt.Sprintf(`
		INSERT INTO %s
			(path, title, artist, album, duration_sec, size_bytes, file_type, is_audio, lrc_path, cover_art_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			title = VALUES(title),
			artist = VALUES(artist),
			album = VALUES(album),
			duration_sec = VALUES(duration_sec),
			size_bytes = VALUES(size_bytes),
			file_type = VALUES(file_type),
			is_audio = VALUES(is_audio),
			lrc_path = VALUES(lrc_path),
			cover_art_path = VALUES(cover_art_path),
			updated_at = CURRENT_TIMESTAMP
	`, musicTable)
	upsertMusicStmt, err := tx.Prepare(upsertMusicSQL)
	if err != nil {
		return stats{}, err
	}
	defer upsertMusicStmt.Close()

	upsertArtistSQL := fmt.Sprintf(`
		INSERT INTO %s (name)
		VALUES (?)
		ON DUPLICATE KEY UPDATE
			updated_at = CURRENT_TIMESTAMP
	`, artistTable)
	upsertArtistStmt, err := tx.Prepare(upsertArtistSQL)
	if err != nil {
		return stats{}, err
	}
	defer upsertArtistStmt.Close()

	upsertLyricsSQL := fmt.Sprintf(`
		INSERT INTO %s (music_path, lrc_path, source)
		VALUES (?, ?, 'catalog')
		ON DUPLICATE KEY UPDATE
			lrc_path = VALUES(lrc_path),
			source = 'catalog',
			updated_at = CURRENT_TIMESTAMP
	`, lyricsTable)
	upsertLyricsStmt, err := tx.Prepare(upsertLyricsSQL)
	if err != nil {
		return stats{}, err
	}
	defer upsertLyricsStmt.Close()

	deleteLyricsSQL := fmt.Sprintf("DELETE FROM %s WHERE music_path = ?", lyricsTable)
	deleteLyricsStmt, err := tx.Prepare(deleteLyricsSQL)
	if err != nil {
		return stats{}, err
	}
	defer deleteLyricsStmt.Close()

	st := stats{}

	for _, f := range all {
		var lrcVal any = nil
		if f.LrcPath != nil && strings.TrimSpace(*f.LrcPath) != "" {
			lrcVal = truncate(strings.TrimSpace(*f.LrcPath), 255)
		}
		var coverVal any = nil
		if f.Cover != nil && strings.TrimSpace(*f.Cover) != "" {
			coverVal = truncate(strings.TrimSpace(*f.Cover), 255)
		}

		if _, err := upsertMusicStmt.Exec(
			f.RelPath,
			truncate(f.Title, 255),
			truncate(f.Artist, 255),
			truncate(f.Album, 255),
			f.Duration,
			f.Size,
			f.FileType,
			boolToTiny(f.IsAudio),
			lrcVal,
			coverVal,
		); err != nil {
			return st, fmt.Errorf("upsert music_files 失败 (%s): %w", f.RelPath, err)
		}
		st.MusicUpserted++

		if f.IsAudio {
			artistSet := splitArtists(f.Artist)
			for _, a := range artistSet {
				if _, err := upsertArtistStmt.Exec(a); err != nil {
					return st, fmt.Errorf("upsert artists 失败 (%s): %w", a, err)
				}
				st.ArtistsUpserted++
			}

			if lrcVal != nil {
				if _, err := upsertLyricsStmt.Exec(f.RelPath, lrcVal); err != nil {
					return st, fmt.Errorf("upsert media_lyrics_map 失败 (%s): %w", f.RelPath, err)
				}
				st.LyricsMapUpserted++
			} else {
				if _, err := deleteLyricsStmt.Exec(f.RelPath); err != nil {
					return st, fmt.Errorf("delete media_lyrics_map 失败 (%s): %w", f.RelPath, err)
				}
				st.LyricsMapDeleted++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return st, err
	}
	return st, nil
}

func ensureMediaLyricsTable(db *sql.DB, mediaSchema string) error {
	createSchema := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARSET=utf8mb4", quoteIdent(mediaSchema))
	if _, err := db.Exec(createSchema); err != nil {
		return err
	}

	lyricsTable := qualifiedName(mediaSchema, "media_lyrics_map")
	createTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT NOT NULL AUTO_INCREMENT,
			music_path VARCHAR(500) NOT NULL,
			lrc_path VARCHAR(500) NOT NULL,
			source VARCHAR(32) NOT NULL DEFAULT 'catalog',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uk_music_path (music_path),
			KEY idx_lrc_path (lrc_path),
			KEY idx_updated_at (updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`, lyricsTable)
	_, err := db.Exec(createTable)
	return err
}

func splitArtists(artist string) []string {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return nil
	}

	replacer := strings.NewReplacer(
		" feat. ", ",",
		" feat ", ",",
		" ft. ", ",",
		" ft ", ",",
		"&", ",",
		"、", ",",
		"，", ",",
		"/", ",",
		"+", ",",
	)
	s := replacer.Replace(artist)
	parts := strings.Split(s, ",")

	seen := make(map[string]struct{}, len(parts)+1)
	out := make([]string, 0, len(parts)+1)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = truncate(p, 255)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func resolvePath(baseDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(baseDir, p))
}

func qualifiedName(schema, table string) string {
	validateIdentOrFail(schema, "schema")
	validateIdentOrFail(table, "table")
	return quoteIdent(schema) + "." + quoteIdent(table)
}

func quoteIdent(s string) string {
	return "`" + s + "`"
}

func validateIdentOrFail(v, label string) {
	if !identPattern.MatchString(v) {
		fatalf("%s 不合法: %q", label, v)
	}
}

func printStats(st stats) {
	fmt.Println("写入统计:")
	fmt.Printf("  音频扫描数: %d\n", st.AudioScanned)
	fmt.Printf("  视频扫描数: %d\n", st.VideoScanned)
	fmt.Printf("  music_files upsert: %d\n", st.MusicUpserted)
	fmt.Printf("  artists upsert: %d\n", st.ArtistsUpserted)
	fmt.Printf("  media_lyrics_map upsert: %d\n", st.LyricsMapUpserted)
	fmt.Printf("  media_lyrics_map delete: %d\n", st.LyricsMapDeleted)
	if st.SkippedTooLongPath > 0 {
		fmt.Printf("  超长路径跳过: %d\n", st.SkippedTooLongPath)
	}
	if st.Warnings > 0 {
		fmt.Printf("  警告数: %d\n", st.Warnings)
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func boolToTiny(v bool) int {
	if v {
		return 1
	}
	return 0
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}
