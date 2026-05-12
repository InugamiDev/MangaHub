package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	mangahubpb "mangahub/services/api/proto"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	APIURL   string `json:"api_url"`
	TCPAddr  string `json:"tcp_addr"`
	UDPAddr  string `json:"udp_addr"`
	GRPCAddr string `json:"grpc_addr"`
	WSURL    string `json:"ws_url"`
	Token    string `json:"token"`
	Username string `json:"username"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return saveConfig(cfg)
	case "version":
		fmt.Println("mangahub cli 0.1.0")
		return nil
	case "server":
		return serverCommand(cfg, args[1:])
	case "config":
		return configCommand(cfg, args[1:])
	case "profile":
		return profileCommand(cfg, args[1:])
	case "stats":
		return statsCommand(cfg, args[1:])
	case "export":
		return exportCommand(cfg, args[1:])
	case "backup":
		return backupCommand(cfg, args[1:])
	case "db":
		return dbCommand(cfg, args[1:])
	case "logs":
		return logsCommand(cfg, args[1:])
	case "update":
		return updateCommand(cfg, args[1:])
	case "auth":
		return authCommand(cfg, args[1:])
	case "manga":
		return mangaCommand(cfg, args[1:])
	case "library":
		return libraryCommand(cfg, args[1:])
	case "admin":
		return adminCommand(cfg, args[1:])
	case "progress":
		return progressCommand(cfg, args[1:])
	case "test":
		return testCommand(cfg, args[1:])
	case "sync":
		return syncCommand(cfg, args[1:])
	case "notify":
		return notifyCommand(cfg, args[1:])
	case "chat":
		return chatCommand(cfg, args[1:])
	case "grpc":
		return grpcCommand(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Println(`mangahub commands:
  init
  version
  server health|status|start|stop|logs
  config show|set|reset
  profile list|create|switch
  stats overview
  export library|progress|all
  backup create|restore
  db check|optimize|stats|repair
  logs errors|search|clean
  update check|install
  auth register --username demo --email demo@example.com
  auth login --username demo
  auth status
  manga search "one piece"
  manga info one-piece
  library add --manga-id one-piece --status reading
  library list
  library remove --manga-id one-piece
  progress update --manga-id one-piece --chapter 2
  test console
  test console --json
  sync monitor
  notify subscribe
  chat send "Hello everyone!"
  chat send "Great chapter!" --manga-id one-piece
  grpc manga get --id one-piece`)
}

func serverCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub server <health|status|start|stop|logs>")
	}
	switch args[0] {
	case "health", "status":
		return printHTTP(cfg, http.MethodGet, "/health", nil, false)
	case "start", "stop", "logs":
		return notImplementedLocalCLI("server " + args[0])
	default:
		return fmt.Errorf("unknown server command %q", args[0])
	}
}

func configCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub config <show|set|reset>")
	}
	switch args[0] {
	case "show":
		cfg.Token = redact(cfg.Token)
		return printValue(cfg)
	case "set":
		if len(args) < 3 {
			return errors.New("usage: mangahub config set <api_url|tcp_addr|udp_addr|grpc_addr|ws_url|username|token> <value>")
		}
		if err := setConfigValue(&cfg, args[1], strings.Join(args[2:], " ")); err != nil {
			return err
		}
		return saveConfig(cfg)
	case "reset":
		return saveConfig(config{APIURL: "http://localhost:8080", TCPAddr: "localhost:9090", UDPAddr: "localhost:9091", GRPCAddr: "localhost:9092", WSURL: "ws://localhost:9093"})
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}

func profileCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 {
		return errors.New("usage: mangahub profile <list|create|switch>")
	}
	switch args[0] {
	case "list":
		fmt.Println("default")
		return nil
	case "create", "switch":
		return notImplementedLocalCLI("profile " + args[0])
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func statsCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 || args[0] != "overview" {
		return errors.New("usage: mangahub stats overview")
	}
	return notImplementedLocalCLI("stats overview")
}

func exportCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub export <library|progress|all>")
	}
	switch args[0] {
	case "library":
		return printHTTP(cfg, http.MethodGet, "/users/library", nil, true)
	case "progress", "all":
		return notImplementedLocalCLI("export " + args[0])
	default:
		return fmt.Errorf("unknown export command %q", args[0])
	}
}

func backupCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 || (args[0] != "create" && args[0] != "restore") {
		return errors.New("usage: mangahub backup <create|restore>")
	}
	return notImplementedLocalCLI("backup " + args[0])
}

func dbCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 {
		return errors.New("usage: mangahub db <check|optimize|stats|repair>")
	}
	switch args[0] {
	case "check", "optimize", "stats", "repair":
		return notImplementedLocalCLI("db " + args[0])
	default:
		return fmt.Errorf("unknown db command %q", args[0])
	}
}

func logsCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 {
		return errors.New("usage: mangahub logs <errors|search|clean>")
	}
	switch args[0] {
	case "errors", "search", "clean":
		return notImplementedLocalCLI("logs " + args[0])
	default:
		return fmt.Errorf("unknown logs command %q", args[0])
	}
}

func updateCommand(cfg config, args []string) error {
	_ = cfg
	if len(args) == 0 || (args[0] != "check" && args[0] != "install") {
		return errors.New("usage: mangahub update <check|install>")
	}
	return notImplementedLocalCLI("update " + args[0])
}

func authCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub auth <register|login|status|logout>")
	}
	switch args[0] {
	case "register":
		fs := flag.NewFlagSet("auth register", flag.ExitOnError)
		username := fs.String("username", "", "username")
		email := fs.String("email", "", "email")
		password := fs.String("password", os.Getenv("MANGAHUB_PASSWORD"), "password")
		_ = fs.Parse(args[1:])
		return printHTTP(cfg, http.MethodPost, "/auth/register", map[string]string{
			"username": *username,
			"email":    *email,
			"password": *password,
		}, false)
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ExitOnError)
		username := fs.String("username", "", "username")
		email := fs.String("email", "", "email")
		password := fs.String("password", os.Getenv("MANGAHUB_PASSWORD"), "password")
		_ = fs.Parse(args[1:])
		payload, err := requestJSON(cfg, http.MethodPost, "/auth/login", map[string]string{
			"username": *username,
			"email":    *email,
			"password": *password,
		}, false)
		if err != nil {
			return err
		}
		var login struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(payload, &login); err != nil {
			return err
		}
		if login.Token == "" {
			return errors.New("login response did not include token")
		}
		cfg.Token = login.Token
		if *username != "" {
			cfg.Username = *username
		}
		if err := saveConfig(cfg); err != nil {
			return err
		}
		return prettyPrint(payload)
	case "status":
		if cfg.Token == "" {
			fmt.Println("not logged in")
			return nil
		}
		fmt.Println("logged in")
		return nil
	case "logout":
		cfg.Token = ""
		return saveConfig(cfg)
	default:
		return fmt.Errorf("unknown auth command %q", args[0])
	}
}

func mangaCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub manga <search|info|list>")
	}
	switch args[0] {
	case "search":
		fs := flag.NewFlagSet("manga search", flag.ExitOnError)
		limit := fs.Int("limit", 24, "result limit")
		_ = fs.Parse(args[1:])
		query := strings.Join(fs.Args(), " ")
		path := "/manga?q=" + url.QueryEscape(query) + "&limit=" + url.QueryEscape(fmt.Sprint(*limit))
		return printHTTP(cfg, http.MethodGet, path, nil, false)
	case "info":
		if len(args) < 2 {
			return errors.New("usage: mangahub manga info <manga-id>")
		}
		return printHTTP(cfg, http.MethodGet, "/manga/"+url.PathEscape(args[1]), nil, false)
	case "list":
		return printHTTP(cfg, http.MethodGet, "/manga?limit=100", nil, false)
	default:
		return fmt.Errorf("unknown manga command %q", args[0])
	}
}

func libraryCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub library <add|list|remove>")
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("library add", flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		statusValue := fs.String("status", "reading", "reading status")
		chapter := fs.Int("chapter", 1, "current chapter")
		_ = fs.Parse(args[1:])
		return printHTTP(cfg, http.MethodPost, "/users/library", map[string]any{
			"manga_id":        *mangaID,
			"status":          *statusValue,
			"current_chapter": *chapter,
		}, true)
	case "list":
		return printHTTP(cfg, http.MethodGet, "/users/library", nil, true)
	case "remove", "delete":
		fs := flag.NewFlagSet("library remove", flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		_ = fs.Parse(args[1:])
		if *mangaID == "" && len(fs.Args()) > 0 {
			*mangaID = fs.Args()[0]
		}
		if *mangaID == "" {
			return errors.New("usage: mangahub library remove --manga-id <id>")
		}
		return printHTTP(cfg, http.MethodDelete, "/users/library/"+url.PathEscape(*mangaID), nil, true)
	default:
		return fmt.Errorf("unknown library command %q", args[0])
	}
}

func adminCommand(cfg config, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: mangahub admin <manga|chapter|notify|sync> <command>")
	}
	switch args[0] {
	case "manga":
		return adminMangaCommand(cfg, args[1:])
	case "chapter":
		return adminChapterCommand(cfg, args[1:])
	case "notify":
		if args[1] != "send" {
			return errors.New("usage: mangahub admin notify send --manga-id <id> --message <text>")
		}
		fs := flag.NewFlagSet("admin notify send", flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		message := fs.String("message", "", "message")
		kind := fs.String("type", "chapter_release", "notification type")
		_ = fs.Parse(args[2:])
		return printAdminHTTP(cfg, http.MethodPost, "/admin/notify", map[string]string{
			"manga_id": *mangaID,
			"message":  *message,
			"type":     *kind,
		})
	case "sync":
		if args[1] != "anilist" {
			return errors.New("usage: mangahub admin sync anilist --query <term> --limit <n>")
		}
		fs := flag.NewFlagSet("admin sync anilist", flag.ExitOnError)
		query := fs.String("query", "", "search query")
		limit := fs.Int("limit", 25, "result limit")
		_ = fs.Parse(args[2:])
		return printAdminHTTP(cfg, http.MethodPost, "/admin/sync/anilist", map[string]any{"query": *query, "limit": *limit})
	default:
		return fmt.Errorf("unknown admin command %q", args[0])
	}
}

func adminMangaCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub admin manga <create|update|delete>")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("admin manga create", flag.ExitOnError)
		id := fs.String("id", "", "manga id")
		title := fs.String("title", "", "title")
		author := fs.String("author", "Unknown", "author")
		genres := fs.String("genres", "", "comma-separated genres")
		statusValue := fs.String("status", "ongoing", "publication status")
		totalChapters := fs.Int("total-chapters", 0, "total chapters")
		description := fs.String("description", "", "description")
		coverURL := fs.String("cover-url", "", "cover url")
		sourceProvider := fs.String("source-provider", "MangaHub admin", "source provider")
		sourceURL := fs.String("source-url", "", "source url")
		rightsStatus := fs.String("rights-status", "admin-managed", "rights status")
		_ = fs.Parse(args[1:])
		return printAdminHTTP(cfg, http.MethodPost, "/admin/manga", map[string]any{
			"id": *id, "title": *title, "author": *author, "genres": splitCSV(*genres), "status": *statusValue,
			"total_chapters": *totalChapters, "description": *description, "cover_url": *coverURL,
			"source_provider": *sourceProvider, "source_url": *sourceURL, "rights_status": *rightsStatus,
		})
	case "update":
		fs := flag.NewFlagSet("admin manga update", flag.ExitOnError)
		id := fs.String("id", "", "manga id")
		title := fs.String("title", "", "title")
		author := fs.String("author", "", "author")
		genres := fs.String("genres", "", "comma-separated genres")
		statusValue := fs.String("status", "", "publication status")
		totalChapters := fs.Int("total-chapters", -1, "total chapters")
		description := fs.String("description", "", "description")
		coverURL := fs.String("cover-url", "", "cover url")
		sourceProvider := fs.String("source-provider", "", "source provider")
		sourceURL := fs.String("source-url", "", "source url")
		rightsStatus := fs.String("rights-status", "", "rights status")
		_ = fs.Parse(args[1:])
		if *id == "" {
			return errors.New("usage: mangahub admin manga update --id <id> [fields]")
		}
		body := map[string]any{}
		addStringField(body, "title", *title)
		addStringField(body, "author", *author)
		if *genres != "" {
			body["genres"] = splitCSV(*genres)
		}
		addStringField(body, "status", *statusValue)
		if *totalChapters >= 0 {
			body["total_chapters"] = *totalChapters
		}
		addStringField(body, "description", *description)
		addStringField(body, "cover_url", *coverURL)
		addStringField(body, "source_provider", *sourceProvider)
		addStringField(body, "source_url", *sourceURL)
		addStringField(body, "rights_status", *rightsStatus)
		return printAdminHTTP(cfg, http.MethodPut, "/admin/manga/"+url.PathEscape(*id), body)
	case "delete":
		fs := flag.NewFlagSet("admin manga delete", flag.ExitOnError)
		id := fs.String("id", "", "manga id")
		_ = fs.Parse(args[1:])
		if *id == "" && len(fs.Args()) > 0 {
			*id = fs.Args()[0]
		}
		if *id == "" {
			return errors.New("usage: mangahub admin manga delete --id <id>")
		}
		return printAdminHTTP(cfg, http.MethodDelete, "/admin/manga/"+url.PathEscape(*id), nil)
	default:
		return fmt.Errorf("unknown admin manga command %q", args[0])
	}
}

func adminChapterCommand(cfg config, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: mangahub admin chapter <create|update|delete>")
	}
	switch args[0] {
	case "create", "update":
		fs := flag.NewFlagSet("admin chapter "+args[0], flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		id := fs.String("id", "", "chapter id")
		title := fs.String("title", "", "title")
		number := fs.Float64("number", 0, "chapter number")
		pages := fs.String("pages", "", "comma-separated page URLs")
		pageCount := fs.Int("page-count", -1, "page count")
		publishStatus := fs.String("publish-status", "", "publish status")
		_ = fs.Parse(args[1:])
		if *mangaID == "" {
			return errors.New("manga-id is required")
		}
		body := map[string]any{}
		if args[0] == "create" {
			addStringField(body, "id", *id)
			addStringField(body, "title", *title)
			body["chapter_number"] = *number
			body["page_urls"] = splitCSV(*pages)
			addStringField(body, "publish_status", firstNonEmpty(*publishStatus, "draft"))
			return printAdminHTTP(cfg, http.MethodPost, "/admin/manga/"+url.PathEscape(*mangaID)+"/chapters", body)
		}
		if *id == "" {
			return errors.New("usage: mangahub admin chapter update --manga-id <id> --id <chapter-id> [fields]")
		}
		addStringField(body, "title", *title)
		if *number > 0 {
			body["chapter_number"] = *number
		}
		if *pages != "" {
			body["page_urls"] = splitCSV(*pages)
		}
		if *pageCount >= 0 {
			body["page_count"] = *pageCount
		}
		addStringField(body, "publish_status", *publishStatus)
		return printAdminHTTP(cfg, http.MethodPut, "/admin/manga/"+url.PathEscape(*mangaID)+"/chapters/"+url.PathEscape(*id), body)
	case "delete":
		fs := flag.NewFlagSet("admin chapter delete", flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		id := fs.String("id", "", "chapter id")
		_ = fs.Parse(args[1:])
		if *mangaID == "" || *id == "" {
			return errors.New("usage: mangahub admin chapter delete --manga-id <id> --id <chapter-id>")
		}
		return printAdminHTTP(cfg, http.MethodDelete, "/admin/manga/"+url.PathEscape(*mangaID)+"/chapters/"+url.PathEscape(*id), nil)
	default:
		return fmt.Errorf("unknown admin chapter command %q", args[0])
	}
}

func progressCommand(cfg config, args []string) error {
	if len(args) == 0 || args[0] != "update" {
		return errors.New("usage: mangahub progress update --manga-id <id> --chapter <number>")
	}
	fs := flag.NewFlagSet("progress update", flag.ExitOnError)
	mangaID := fs.String("manga-id", "", "manga id")
	chapter := fs.Int("chapter", 0, "chapter")
	statusValue := fs.String("status", "reading", "reading status")
	_ = fs.Parse(args[1:])
	return printHTTP(cfg, http.MethodPut, "/users/progress", map[string]any{
		"manga_id": *mangaID,
		"chapter":  *chapter,
		"status":   *statusValue,
	}, true)
}

type consoleMetric struct {
	Name       string `json:"name"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
	Error      string `json:"error,omitempty"`
}

func testCommand(cfg config, args []string) error {
	if len(args) == 0 || args[0] != "console" {
		return errors.New("usage: mangahub test console [--json] [--skip-protocols]")
	}
	fs := flag.NewFlagSet("test console", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "print JSON metrics")
	skipProtocols := fs.Bool("skip-protocols", false, "only test HTTP API")
	_ = fs.Parse(args[1:])

	metrics := []consoleMetric{
		measureHTTP(cfg, "http.health", http.MethodGet, "/health", nil, false),
		measureHTTP(cfg, "http.manga_search", http.MethodGet, "/manga?q=one&limit=5", nil, false),
	}
	if !*skipProtocols {
		metrics = append(metrics,
			measureGRPC(cfg),
			measureUDP(cfg),
			measureTCP(cfg),
			measureWebSocket(cfg),
		)
	}
	if *jsonOutput {
		return printValue(metrics)
	}
	printConsoleMetrics(metrics)
	return nil
}

func syncCommand(cfg config, args []string) error {
	if len(args) == 0 || args[0] != "monitor" {
		return errors.New("usage: mangahub sync monitor")
	}
	if cfg.Token == "" {
		return errors.New("login required")
	}
	conn, err := net.Dial("tcp", cfg.TCPAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(map[string]string{"type": "auth", "token": cfg.Token}); err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, conn)
	return err
}

func notifyCommand(cfg config, args []string) error {
	if len(args) == 0 || args[0] != "subscribe" {
		return errors.New("usage: mangahub notify subscribe")
	}
	conn, err := net.Dial("udp", cfg.UDPAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(map[string]string{"type": "register"}); err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, conn)
	return err
}

func chatCommand(cfg config, args []string) error {
	if len(args) == 0 || args[0] != "send" {
		return errors.New(`usage: mangahub chat send "Hello everyone!"`)
	}
	if cfg.Token == "" {
		return errors.New("login required")
	}
	room, message := parseChatSend(args[1:])
	if message == "" {
		return errors.New("message is required")
	}
	chatURL := cfg.WSURL + "/ws/chat?token=" + url.QueryEscape(cfg.Token) + "&room=" + url.QueryEscape(room)
	conn, _, err := websocket.DefaultDialer.Dial(chatURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]string{"type": "message", "message": message}); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var payload any
		if err := conn.ReadJSON(&payload); err != nil {
			return nil
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(data))
	}
}

func parseChatSend(args []string) (string, string) {
	room := "global"
	messageParts := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--room":
			if i+1 < len(args) {
				room = args[i+1]
				i++
			}
		case "--manga-id":
			if i+1 < len(args) {
				room = "manga:" + args[i+1]
				i++
			}
		case "--message":
			if i+1 < len(args) {
				messageParts = append(messageParts, args[i+1])
				i++
			}
		default:
			messageParts = append(messageParts, args[i])
		}
	}
	return room, strings.TrimSpace(strings.Join(messageParts, " "))
}

func grpcCommand(cfg config, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: mangahub grpc <manga|progress> <command>")
	}
	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	// intent: exercise the protobuf-generated gRPC client instead of the retired JSON codec path
	// status: done
	// next: regenerate services/api/proto/*.pb.go after proto contract changes
	// blockers: none
	// confidence: high
	client := mangahubpb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch args[0] {
	case "manga":
		switch args[1] {
		case "get":
			fs := flag.NewFlagSet("grpc manga get", flag.ExitOnError)
			id := fs.String("id", "", "manga id")
			_ = fs.Parse(args[2:])
			resp, err := client.GetManga(ctx, &mangahubpb.GetMangaRequest{Id: *id})
			if err != nil {
				return err
			}
			return printValue(resp)
		case "search":
			fs := flag.NewFlagSet("grpc manga search", flag.ExitOnError)
			query := fs.String("query", "", "query")
			limit := fs.Int("limit", 24, "limit")
			_ = fs.Parse(args[2:])
			resp, err := client.SearchManga(ctx, &mangahubpb.SearchRequest{Query: *query, Limit: int32(*limit)})
			if err != nil {
				return err
			}
			return printValue(resp)
		default:
			return fmt.Errorf("unknown grpc manga command %q", args[1])
		}
	case "progress":
		if args[1] != "update" {
			return errors.New("usage: mangahub grpc progress update --manga-id <id> --chapter <number>")
		}
		if cfg.Token == "" {
			return errors.New("login required")
		}
		fs := flag.NewFlagSet("grpc progress update", flag.ExitOnError)
		mangaID := fs.String("manga-id", "", "manga id")
		chapter := fs.Int("chapter", 0, "chapter")
		statusValue := fs.String("status", "reading", "status")
		_ = fs.Parse(args[2:])
		resp, err := client.UpdateProgress(ctx, &mangahubpb.ProgressRequest{
			Token: cfg.Token, MangaId: *mangaID, Chapter: int32(*chapter), Status: *statusValue,
		})
		if err != nil {
			return err
		}
		return printValue(resp)
	default:
		return fmt.Errorf("unknown grpc command %q", args[0])
	}
}

func printHTTP(cfg config, method, path string, body any, authRequired bool) error {
	payload, err := requestJSON(cfg, method, path, body, authRequired)
	if err != nil {
		return err
	}
	return prettyPrint(payload)
}

func printAdminHTTP(cfg config, method, path string, body any) error {
	payload, err := requestAdminJSON(cfg, method, path, body)
	if err != nil {
		return err
	}
	return prettyPrint(payload)
}

func requestJSON(cfg config, method, path string, body any, authRequired bool) ([]byte, error) {
	headers := map[string]string{}
	if authRequired {
		if cfg.Token == "" {
			return nil, errors.New("login required")
		}
		headers["Authorization"] = "Bearer " + cfg.Token
	}
	return requestJSONWithHeaders(cfg, method, path, body, headers)
}

func requestAdminJSON(cfg config, method, path string, body any) ([]byte, error) {
	token := os.Getenv("MANGAHUB_ADMIN_TOKEN")
	if token == "" {
		token = os.Getenv("ADMIN_SYNC_TOKEN")
	}
	if token == "" {
		return nil, errors.New("MANGAHUB_ADMIN_TOKEN or ADMIN_SYNC_TOKEN is required")
	}
	return requestJSONWithHeaders(cfg, method, path, body, map[string]string{"X-Admin-Sync-Token": token})
}

func requestJSONWithHeaders(cfg config, method, path string, body any, headers map[string]string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, joinURL(cfg.APIURL, path), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return payload, nil
}

func loadConfig() (config, error) {
	cfg := config{
		APIURL:   getenv("MANGAHUB_API_URL", "http://localhost:8080"),
		TCPAddr:  getenv("MANGAHUB_TCP_ADDR", "localhost:9090"),
		UDPAddr:  getenv("MANGAHUB_UDP_ADDR", "localhost:9091"),
		GRPCAddr: getenv("MANGAHUB_GRPC_ADDR", "localhost:9092"),
		WSURL:    getenv("MANGAHUB_WS_URL", "ws://localhost:9093"),
	}
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer file.Close()
	if err := parseManualConfig(file, &cfg); err != nil {
		return cfg, err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8080"
	}
	if cfg.TCPAddr == "" {
		cfg.TCPAddr = "localhost:9090"
	}
	if cfg.UDPAddr == "" {
		cfg.UDPAddr = "localhost:9091"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = "localhost:9092"
	}
	if cfg.WSURL == "" {
		cfg.WSURL = "ws://localhost:9093"
	}
	return cfg, nil
}

func saveConfig(cfg config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(renderManualConfig(cfg)), 0o600); err != nil {
		return err
	}
	fmt.Println("saved", path)
	return nil
}

func setConfigValue(cfg *config, key, value string) error {
	switch key {
	case "api_url":
		cfg.APIURL = value
	case "tcp_addr":
		cfg.TCPAddr = value
	case "udp_addr":
		cfg.UDPAddr = value
	case "grpc_addr":
		cfg.GRPCAddr = value
	case "ws_url":
		cfg.WSURL = value
	case "username":
		cfg.Username = value
	case "token":
		cfg.Token = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	return nil
}

func redact(value string) string {
	if value == "" {
		return ""
	}
	return "<redacted>"
}

func notImplementedLocalCLI(command string) error {
	return fmt.Errorf("%s is recognized but not implemented in the local CLI", command)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mangahub", "config.yaml"), nil
}

func parseManualConfig(reader io.Reader, cfg *config) error {
	scanner := bufio.NewScanner(reader)
	section := ""
	host := "localhost"
	ports := map[string]int{"http": 8080, "tcp": 9090, "udp": 9091, "grpc": 9092, "websocket": 9093}
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch section {
		case "server":
			switch key {
			case "host":
				host = value
			case "http_port":
				ports["http"] = parsePort(value, ports["http"])
			case "tcp_port":
				ports["tcp"] = parsePort(value, ports["tcp"])
			case "udp_port":
				ports["udp"] = parsePort(value, ports["udp"])
			case "grpc_port":
				ports["grpc"] = parsePort(value, ports["grpc"])
			case "websocket_port":
				ports["websocket"] = parsePort(value, ports["websocket"])
			}
		case "user":
			switch key {
			case "username":
				cfg.Username = value
			case "token":
				cfg.Token = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	cfg.APIURL = fmt.Sprintf("http://%s:%d", host, ports["http"])
	cfg.TCPAddr = fmt.Sprintf("%s:%d", host, ports["tcp"])
	cfg.UDPAddr = fmt.Sprintf("%s:%d", host, ports["udp"])
	cfg.GRPCAddr = fmt.Sprintf("%s:%d", host, ports["grpc"])
	cfg.WSURL = fmt.Sprintf("ws://%s:%d", host, ports["websocket"])
	return nil
}

func renderManualConfig(cfg config) string {
	host := hostFromURL(cfg.APIURL, "localhost")
	httpPort := portFromURL(cfg.APIURL, 8080)
	tcpPort := portFromAddr(cfg.TCPAddr, 9090)
	udpPort := portFromAddr(cfg.UDPAddr, 9091)
	grpcPort := portFromAddr(cfg.GRPCAddr, 9092)
	websocketPort := portFromURL(cfg.WSURL, 9093)
	return fmt.Sprintf(`server:
  host: "%s"
  http_port: %d
  tcp_port: %d
  udp_port: %d
  grpc_port: %d
  websocket_port: %d

database:
  path: "~/.mangahub/data.db"

user:
  username: "%s"
  token: "%s"

sync:
  auto_sync: true
  conflict_resolution: "last_write_wins"

notifications:
  enabled: true
  sound: false

logging:
  level: "info"
  path: "~/.mangahub/logs/"
`, host, httpPort, tcpPort, udpPort, grpcPort, websocketPort, cfg.Username, cfg.Token)
}

func parsePort(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func hostFromURL(rawURL, fallback string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return fallback
	}
	return parsed.Hostname()
}

func portFromURL(rawURL string, fallback int) int {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Port() == "" {
		return fallback
	}
	return parsePort(parsed.Port(), fallback)
}

func portFromAddr(addr string, fallback int) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fallback
	}
	return parsePort(port, fallback)
}

func prettyPrint(payload []byte) error {
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		fmt.Println(string(payload))
		return nil
	}
	return printValue(decoded)
}

func printValue(value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func measureHTTP(cfg config, name, method, path string, body any, authRequired bool) consoleMetric {
	start := time.Now()
	payload, err := requestJSON(cfg, method, path, body, authRequired)
	metric := consoleMetric{Name: name, Target: joinURL(cfg.APIURL, path), DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		metric.Status = "fail"
		metric.Error = err.Error()
		return metric
	}
	metric.Status = "pass"
	var decoded map[string]any
	if json.Unmarshal(payload, &decoded) == nil {
		if count, ok := decoded["count"]; ok {
			metric.Detail = fmt.Sprintf("count=%v", count)
		} else if statusValue, ok := decoded["status"]; ok {
			metric.Detail = fmt.Sprintf("status=%v", statusValue)
		}
	}
	return metric
}

func measureGRPC(cfg config) consoleMetric {
	start := time.Now()
	metric := consoleMetric{Name: "grpc.search", Target: cfg.GRPCAddr}
	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return finishMetric(metric, start, err, "")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := mangahubpb.NewMangaServiceClient(conn)
	resp, err := client.SearchManga(ctx, &mangahubpb.SearchRequest{Query: "one", Limit: 5})
	count := int32(0)
	if resp != nil {
		count = resp.GetCount()
	}
	return finishMetric(metric, start, err, fmt.Sprintf("count=%d", count))
}

func measureUDP(cfg config) consoleMetric {
	start := time.Now()
	metric := consoleMetric{Name: "udp.ping", Target: cfg.UDPAddr}
	conn, err := net.Dial("udp", cfg.UDPAddr)
	if err != nil {
		return finishMetric(metric, start, err, "")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if err := json.NewEncoder(conn).Encode(map[string]string{"type": "ping"}); err != nil {
		return finishMetric(metric, start, err, "")
	}
	var notification map[string]any
	if err := json.NewDecoder(conn).Decode(&notification); err != nil {
		return finishMetric(metric, start, err, "")
	}
	return finishMetric(metric, start, nil, fmt.Sprintf("type=%v", notification["type"]))
}

func measureTCP(cfg config) consoleMetric {
	start := time.Now()
	metric := consoleMetric{Name: "tcp.auth_ping", Target: cfg.TCPAddr}
	if cfg.Token == "" {
		return skipMetric(metric, start, "login required for TCP auth test")
	}
	conn, err := net.DialTimeout("tcp", cfg.TCPAddr, 3*time.Second)
	if err != nil {
		return finishMetric(metric, start, err, "")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)
	if err := encoder.Encode(map[string]string{"type": "auth", "token": cfg.Token}); err != nil {
		return finishMetric(metric, start, err, "")
	}
	var ready map[string]any
	if err := decoder.Decode(&ready); err != nil {
		return finishMetric(metric, start, err, "")
	}
	if err := encoder.Encode(map[string]string{"type": "ping"}); err != nil {
		return finishMetric(metric, start, err, "")
	}
	var pong map[string]any
	if err := decoder.Decode(&pong); err != nil {
		return finishMetric(metric, start, err, "")
	}
	return finishMetric(metric, start, nil, fmt.Sprintf("ready=%v ping=%v", ready["type"], pong["type"]))
}

func measureWebSocket(cfg config) consoleMetric {
	start := time.Now()
	metric := consoleMetric{Name: "websocket.ping", Target: cfg.WSURL + "/ws/chat"}
	if cfg.Token == "" {
		return skipMetric(metric, start, "login required for WebSocket auth test")
	}
	chatURL := cfg.WSURL + "/ws/chat?token=" + url.QueryEscape(cfg.Token) + "&room=console"
	conn, _, err := websocket.DefaultDialer.Dial(chatURL, nil)
	if err != nil {
		return finishMetric(metric, start, err, "")
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		return finishMetric(metric, start, err, "")
	}
	for {
		var payload map[string]any
		if err := conn.ReadJSON(&payload); err != nil {
			return finishMetric(metric, start, err, "")
		}
		if payload["type"] == "pong" {
			return finishMetric(metric, start, nil, "type=pong")
		}
	}
}

func finishMetric(metric consoleMetric, start time.Time, err error, detail string) consoleMetric {
	metric.DurationMS = time.Since(start).Milliseconds()
	metric.Detail = detail
	if err != nil {
		metric.Status = "fail"
		metric.Error = err.Error()
		return metric
	}
	metric.Status = "pass"
	return metric
}

func skipMetric(metric consoleMetric, start time.Time, detail string) consoleMetric {
	metric.DurationMS = time.Since(start).Milliseconds()
	metric.Status = "skip"
	metric.Detail = detail
	return metric
}

func printConsoleMetrics(metrics []consoleMetric) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "CHECK\tSTATUS\tMS\tTARGET\tDETAIL")
	for _, metric := range metrics {
		detail := metric.Detail
		if metric.Error != "" {
			detail = metric.Error
		}
		fmt.Fprintf(writer, "%s\t%s\t%d\t%s\t%s\n", metric.Name, metric.Status, metric.DurationMS, metric.Target, detail)
	}
	_ = writer.Flush()
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func addStringField(body map[string]any, key, value string) {
	if strings.TrimSpace(value) != "" {
		body[key] = strings.TrimSpace(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func joinURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
