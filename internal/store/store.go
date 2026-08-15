package store

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ytmee/litt/internal/graph"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Label struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

// ErrNotFound wraps errors returned when a requested row does not exist, so
// callers can distinguish missing entities from store failures.
var ErrNotFound = errors.New("not found")

var seedLabels = []Label{
	{Name: "needs-triage", Color: "e4e669", Description: "Maintainer needs to evaluate this issue", Kind: "triage"},
	{Name: "needs-info", Color: "fef2c0", Description: "Waiting on reporter for more information", Kind: "triage"},
	{Name: "ready-for-agent", Color: "006B75", Description: "Fully specified, ready for an AFK agent", Kind: "triage"},
	{Name: "ready-for-human", Color: "bfdadc", Description: "Requires human implementation", Kind: "triage"},
	{Name: "wontfix", Color: "ffffff", Description: "Will not be actioned", Kind: "triage"},
	{Name: "enhancement", Color: "a2eeef", Description: "New feature or request", Kind: "category"},
}

type Querier interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

type Store struct {
	writeDB *sql.DB
	readDB  *sql.DB
}

func openWriteDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_txlock=immediate", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open write db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

func openReadDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)
	return db, nil
}

func Open(path string) (*Store, error) {
	writeDB, err := openWriteDB(path)
	if err != nil {
		return nil, err
	}
	readDB, err := openReadDB(path)
	if err != nil {
		writeDB.Close()
		return nil, err
	}
	return &Store{writeDB: writeDB, readDB: readDB}, nil
}

func OpenInMemory() (*Store, error) {
	db, err := sql.Open("sqlite", "file:litt.db?mode=memory&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open in-memory db: %w", err)
	}
	return &Store{writeDB: db, readDB: db}, nil
}

func (s *Store) Close() error {
	if s.readDB == s.writeDB {
		return s.writeDB.Close()
	}
	rerr := s.readDB.Close()
	werr := s.writeDB.Close()
	if rerr != nil && werr != nil {
		return fmt.Errorf("close read: %v; close write: %v", rerr, werr)
	}
	if rerr != nil {
		return rerr
	}
	return werr
}

type migration struct {
	version int
	sql     string
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var migrations []migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		var version int
		if _, err := fmt.Sscanf(entry.Name(), "%d_", &version); err != nil {
			continue
		}
		migrations = append(migrations, migration{version: version, sql: string(data)})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func (s *Store) Migrate() error {
	return retryWrite("Migrate", func() error {
		migrations, err := loadMigrations()
		if err != nil {
			return err
		}
		if _, err := s.writeDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
		for _, m := range migrations {
			err := s.writeDB.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", m.version).Scan(new(int))
			if err == sql.ErrNoRows {
				if _, err := s.writeDB.Exec(m.sql); err != nil {
					return fmt.Errorf("apply migration %d: %w", m.version, err)
				}
				if _, err := s.writeDB.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
					return fmt.Errorf("record migration %d: %w", m.version, err)
				}
			} else if err != nil {
				return fmt.Errorf("check migration %d: %w", m.version, err)
			}
		}
		return nil
	})
}

func (s *Store) SeedLabels() error {
	return retryWrite("SeedLabels", func() error {
		for _, l := range seedLabels {
			_, err := s.writeDB.Exec(
				"INSERT OR IGNORE INTO labels (name, color, description, kind) VALUES (?, ?, ?, ?)",
				l.Name, l.Color, l.Description, l.Kind,
			)
			if err != nil {
				return fmt.Errorf("seed label %s: %w", l.Name, err)
			}
		}
		return nil
	})
}

func (s *Store) CreateLabel(name, description, kind string) (*Label, error) {
	return retryVal("CreateLabel", func() (*Label, error) {
		if name == "" {
			return nil, fmt.Errorf("label name is required")
		}
		validKinds := map[string]bool{"triage": true, "category": true, "custom": true}
		if !validKinds[kind] {
			return nil, fmt.Errorf("invalid label kind %q: must be one of triage, category, or custom", kind)
		}
		_, err := s.writeDB.Exec(
			"INSERT INTO labels (name, color, description, kind) VALUES (?, 'ffffff', ?, ?)",
			name, description, kind,
		)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return nil, fmt.Errorf("label %q already exists", name)
			}
			return nil, fmt.Errorf("create label %s: %w", name, err)
		}
		return s.FindLabel(name)
	})
}

func (s *Store) DeleteLabel(name string) error {
	return retryWrite("DeleteLabel", func() error {
		result, err := s.writeDB.Exec("DELETE FROM labels WHERE name = ?", name)
		if err != nil {
			return fmt.Errorf("delete label %s: %w", name, err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return fmt.Errorf("label %q not found", name)
		}
		return nil
	})
}

func (s *Store) ListLabels() ([]Label, error) {
	rows, err := s.readDB.Query("SELECT id, name, color, description, kind FROM labels ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	defer rows.Close()
	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.Color, &l.Description, &l.Kind); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	jitterPct   float64
}

var defaultRetry = retryConfig{
	maxAttempts: 6,
	baseDelay:   50 * time.Millisecond,
	maxDelay:    5 * time.Second,
	jitterPct:   0.25,
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "SQLITE_LOCKED")
}

func retryBackoff(attempt int, cfg retryConfig) time.Duration {
	delay := float64(cfg.baseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(cfg.maxDelay) {
		delay = float64(cfg.maxDelay)
	}
	jitter := delay * cfg.jitterPct * (2*rand.Float64() - 1)
	return time.Duration(delay + jitter)
}

func retryWrite(opName string, fn func() error) error {
	var err error
	for attempt := 1; attempt <= defaultRetry.maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if !isTransientError(err) {
			return err
		}
		if attempt == defaultRetry.maxAttempts {
			break
		}
		time.Sleep(retryBackoff(attempt, defaultRetry))
	}
	return fmt.Errorf("%s: %w after %d attempts", opName, err, defaultRetry.maxAttempts)
}

func retryVal[T any](opName string, fn func() (T, error)) (T, error) {
	var zero T
	var val T
	err := retryWrite(opName, func() (err error) {
		val, err = fn()
		return err
	})
	if err != nil {
		return zero, err
	}
	return val, nil
}

type Comment struct {
	ID        int    `json:"id"`
	IssueID   int    `json:"issue_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type Issue struct {
	ID            int      `json:"number"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	State         string   `json:"state"`
	Kind          string   `json:"kind"`
	ParentIssueID *int     `json:"parent_issue_id"`
	Labels        []Label  `json:"labels"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	ClosedAt      *string  `json:"closed_at"`
}

func (i Issue) MarshalJSON() ([]byte, error) {
	type issueAlias Issue
	a := issueAlias(i)
	if a.Labels == nil {
		a.Labels = []Label{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(struct {
		issueAlias
		Ref string `json:"ref"`
	}{
		issueAlias: a,
		Ref:        fmt.Sprintf("#%d", i.ID),
	})
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

type UpdateIssueOptions struct {
	Title        *string  `json:"title,omitempty"`
	Body         *string  `json:"body,omitempty"`
	State        *string  `json:"state,omitempty"`
	Kind         *string  `json:"kind,omitempty"`
	AddLabels    []string `json:"add_labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
}

func (s *Store) FindLabel(name string) (*Label, error) {
	var l Label
	err := s.writeDB.QueryRow(
		"SELECT id, name, color, description, kind FROM labels WHERE name = ?", name,
	).Scan(&l.ID, &l.Name, &l.Color, &l.Description, &l.Kind)
	if err != nil {
		return nil, fmt.Errorf("find label %s: %w", name, err)
	}
	return &l, nil
}

func (s *Store) GetOrCreateLabel(name string) (*Label, error) {
	return retryVal("GetOrCreateLabel", func() (*Label, error) {
		l, err := s.FindLabel(name)
		if err == nil {
			return l, nil
		}

		_, execErr := s.writeDB.Exec(
			"INSERT OR IGNORE INTO labels (name, color, description, kind) VALUES (?, 'ffffff', '', 'custom')", name,
		)
		if execErr != nil {
			return nil, fmt.Errorf("create label %s: %w", name, execErr)
		}

		l, err = s.FindLabel(name)
		if err != nil {
			return nil, fmt.Errorf("re-query label %s: %w", name, err)
		}
		return l, nil
	})
}

func (s *Store) getIssueLabels(q Querier, issueID int) ([]Label, error) {
	rows, err := q.Query(
		`SELECT l.id, l.name, l.color, l.description, l.kind
		 FROM labels l
		 JOIN issue_labels il ON l.id = il.label_id
		 WHERE il.issue_id = ?
		 ORDER BY l.id`, issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("get labels for issue %d: %w", issueID, err)
	}
	defer rows.Close()
	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.Name, &l.Color, &l.Description, &l.Kind); err != nil {
			return nil, fmt.Errorf("scan label: %w", err)
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

var ValidKinds = map[string]bool{
	"spec": true,
	"task": true,
	"bug":  true,
}

func (s *Store) attachLabel(q Querier, issueID int, label *Label) error {
	if label.Kind == "triage" {
		_, err := q.Exec(
			`DELETE FROM issue_labels
			 WHERE issue_id = ? AND label_id IN (
				 SELECT id FROM labels WHERE kind = 'triage'
			 )`, issueID,
		)
		if err != nil {
			return fmt.Errorf("remove existing triage labels: %w", err)
		}
	}
	_, err := q.Exec(
		"INSERT OR IGNORE INTO issue_labels (issue_id, label_id) VALUES (?, ?)",
		issueID, label.ID,
	)
	if err != nil {
		return fmt.Errorf("attach label %q: %w", label.Name, err)
	}
	return nil
}

func validateKind(kind string) error {
	if !ValidKinds[kind] {
		return fmt.Errorf("invalid kind %q: must be one of spec, task, or bug", kind)
	}
	return nil
}

func (s *Store) CreateIssue(title, kind, body string, labelNames []string) (issue *Issue, err error) {
	err = retryWrite("CreateIssue", func() error {
		if title == "" {
			return fmt.Errorf("title is required")
		}
		if err := validateKind(kind); err != nil {
			return err
		}
		result, execErr := s.writeDB.Exec(
			"INSERT INTO issues (title, kind, body) VALUES (?, ?, ?)",
			title, kind, body,
		)
		if execErr != nil {
			return fmt.Errorf("create issue: %w", execErr)
		}
		id, _ := result.LastInsertId()
		intID := int(id)

		for _, name := range labelNames {
			label, findErr := s.FindLabel(name)
			if findErr != nil {
				return fmt.Errorf("label %q does not exist", name)
			}
			if execErr = s.attachLabel(s.writeDB, intID, label); execErr != nil {
				return execErr
			}
		}

		issue, execErr = s.GetIssue(intID)
		return execErr
	})
	return
}

func (s *Store) GetIssue(id int) (*Issue, error) {
	row := s.readDB.QueryRow(
		`SELECT id, title, body, state, kind, parent_issue_id, created_at, updated_at, closed_at
		 FROM issues WHERE id = ?`, id,
	)
	var issue Issue
	err := row.Scan(&issue.ID, &issue.Title, &issue.Body, &issue.State, &issue.Kind, &issue.ParentIssueID, &issue.CreatedAt, &issue.UpdatedAt, &issue.ClosedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("issue %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get issue %d: %w", id, err)
	}
	labels, err := s.getIssueLabels(s.readDB, issue.ID)
	if err != nil {
		return nil, err
	}
	issue.Labels = labels
	return &issue, nil
}

func (s *Store) ListIssues(state, kind, label string, parentID *int, isBlocked *bool) ([]Issue, error) {
	query := `SELECT DISTINCT i.id, i.title, i.body, i.state, i.kind, i.parent_issue_id, i.created_at, i.updated_at, i.closed_at FROM issues i`
	var args []interface{}
	var conditions []string

	if isBlocked != nil {
		query += ` LEFT JOIN issue_blocks ib ON i.id = ib.blocked_issue_id`
		query += ` LEFT JOIN issues blocker ON ib.blocker_issue_id = blocker.id AND blocker.state = 'open'`
	}

	if label != "" {
		query += ` JOIN issue_labels il ON i.id = il.issue_id JOIN labels l ON il.label_id = l.id`
		conditions = append(conditions, "l.name = ?")
		args = append(args, label)
	}
	if state != "" {
		conditions = append(conditions, "i.state = ?")
		args = append(args, state)
	}
	if kind != "" {
		conditions = append(conditions, "i.kind = ?")
		args = append(args, kind)
	}
	if parentID != nil {
		if *parentID == 0 {
			conditions = append(conditions, "i.parent_issue_id IS NULL")
		} else {
			conditions = append(conditions, "i.parent_issue_id = ?")
			args = append(args, *parentID)
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	if isBlocked != nil {
		query += " GROUP BY i.id"
		if *isBlocked {
			query += " HAVING COUNT(blocker.id) > 0"
		} else {
			query += " HAVING COUNT(blocker.id) = 0"
		}
	}
	query += " ORDER BY i.id"

	rows, err := s.readDB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()
	issues := make([]Issue, 0)
	for rows.Next() {
		var i Issue
		if err := rows.Scan(&i.ID, &i.Title, &i.Body, &i.State, &i.Kind, &i.ParentIssueID, &i.CreatedAt, &i.UpdatedAt, &i.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}
		issues = append(issues, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range issues {
		labels, err := s.getIssueLabels(s.readDB, issues[idx].ID)
		if err != nil {
			return nil, err
		}
		issues[idx].Labels = labels
	}
	return issues, nil
}

func (s *Store) UpdateIssue(id int, opts UpdateIssueOptions) error {
	return retryWrite("UpdateIssue", func() error {
		if _, err := s.GetIssue(id); err != nil {
			return err
		}
		var setClauses []string
		var args []interface{}

		if opts.Title != nil {
			setClauses = append(setClauses, "title = ?")
			args = append(args, *opts.Title)
		}
		if opts.Body != nil {
			setClauses = append(setClauses, "body = ?")
			args = append(args, *opts.Body)
		}
		if opts.Kind != nil {
			if err := validateKind(*opts.Kind); err != nil {
				return err
			}
			setClauses = append(setClauses, "kind = ?")
			args = append(args, *opts.Kind)
		}
		if opts.State != nil {
			setClauses = append(setClauses, "state = ?")
			args = append(args, *opts.State)
			if *opts.State == "closed" {
				setClauses = append(setClauses, "closed_at = datetime('now')")
			} else if *opts.State == "open" {
				setClauses = append(setClauses, "closed_at = NULL")
			}
		}

		if len(setClauses) == 0 && len(opts.AddLabels) == 0 && len(opts.RemoveLabels) == 0 {
			return fmt.Errorf("no fields to update")
		}

		if len(setClauses) > 0 {
			setClauses = append(setClauses, "updated_at = datetime('now')")
			query := "UPDATE issues SET " + strings.Join(setClauses, ", ") + " WHERE id = ?"
			args = append(args, id)
			_, err := s.writeDB.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("update issue %d: %w", id, err)
			}
		}

		if len(opts.AddLabels) > 0 || len(opts.RemoveLabels) > 0 {
			if err := s.UpdateIssueLabels(id, opts.AddLabels, opts.RemoveLabels); err != nil {
				return err
			}
			if len(setClauses) == 0 {
				_, err := s.writeDB.Exec("UPDATE issues SET updated_at = datetime('now') WHERE id = ?", id)
				if err != nil {
					return fmt.Errorf("update issue timestamp: %w", err)
				}
			}
		}

		return nil
	})
}

func (s *Store) UpdateIssueLabels(issueID int, addLabels, removeLabels []string) error {
	return retryWrite("UpdateIssueLabels", func() error {
		for _, name := range removeLabels {
			_, err := s.writeDB.Exec(
				"DELETE FROM issue_labels WHERE issue_id = ? AND label_id IN (SELECT id FROM labels WHERE name = ?)",
				issueID, name,
			)
			if err != nil {
				return fmt.Errorf("remove label %s: %w", name, err)
			}
		}

		for _, name := range addLabels {
			label, err := s.FindLabel(name)
			if err != nil {
				return fmt.Errorf("label %q does not exist", name)
			}
			if err = s.attachLabel(s.writeDB, issueID, label); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SetParent(id, parentID int) error {
	return retryWrite("SetParent", func() error {
		if id == parentID {
			return fmt.Errorf("issue cannot be its own parent")
		}

		if _, err := s.GetIssue(id); err != nil {
			return err
		}
		parent, err := s.GetIssue(parentID)
		if err != nil {
			return err
		}
		if parent.State != "open" {
			return fmt.Errorf("parent issue must be open")
		}

		cycle, err := graph.HasPath(parentID, id, func(node int) ([]int, error) {
			var next *int
			err := s.writeDB.QueryRow("SELECT parent_issue_id FROM issues WHERE id = ?", node).Scan(&next)
			if err != nil {
				return nil, fmt.Errorf("check parent cycle: %w", err)
			}
			if next == nil {
				return nil, nil
			}
			return []int{*next}, nil
		})
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("setting parent would create a cycle")
		}

		_, err = s.writeDB.Exec("UPDATE issues SET parent_issue_id = ?, updated_at = datetime('now') WHERE id = ?", parentID, id)
		if err != nil {
			return fmt.Errorf("set parent: %w", err)
		}
		return nil
	})
}

func (s *Store) ClearParent(id int) error {
	return retryWrite("ClearParent", func() error {
		if _, err := s.GetIssue(id); err != nil {
			return err
		}

		_, err := s.writeDB.Exec("UPDATE issues SET parent_issue_id = NULL, updated_at = datetime('now') WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("clear parent: %w", err)
		}
		return nil
	})
}

func (s *Store) ListChildren(parentID int) ([]Issue, error) {
	return s.ListIssues("", "", "", &parentID, nil)
}

func (s *Store) CreateBlock(blockerID, blockedID int) (created bool, err error) {
	err = retryWrite("CreateBlock", func() (rerr error) {
		if blockerID == blockedID {
			return fmt.Errorf("issue cannot block itself")
		}

		if _, err := s.GetIssue(blockerID); err != nil {
			return err
		}
		if _, err := s.GetIssue(blockedID); err != nil {
			return err
		}

		cycle, err := graph.HasPath(blockedID, blockerID, func(node int) ([]int, error) {
			rows, err := s.writeDB.Query("SELECT blocked_issue_id FROM issue_blocks WHERE blocker_issue_id = ?", node)
			if err != nil {
				return nil, fmt.Errorf("check block cycle: %w", err)
			}
			defer rows.Close()
			var ids []int
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					return nil, fmt.Errorf("scan blocked issue: %w", err)
				}
				ids = append(ids, id)
			}
			return ids, rows.Err()
		})
		if err != nil {
			return err
		}
		if cycle {
			return fmt.Errorf("blocking this issue would create a cycle")
		}

		result, execErr := s.writeDB.Exec(
			"INSERT OR IGNORE INTO issue_blocks (blocker_issue_id, blocked_issue_id) VALUES (?, ?)",
			blockerID, blockedID,
		)
		if execErr != nil {
			return fmt.Errorf("create block: %w", execErr)
		}
		n, _ := result.RowsAffected()
		created = n > 0
		return nil
	})
	return
}

func (s *Store) RemoveBlock(blockerID, blockedID int) error {
	return retryWrite("RemoveBlock", func() error {
		result, err := s.writeDB.Exec(
			"DELETE FROM issue_blocks WHERE blocker_issue_id = ? AND blocked_issue_id = ?",
			blockerID, blockedID,
		)
		if err != nil {
			return fmt.Errorf("remove block: %w", err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return fmt.Errorf("block edge not found")
		}
		return nil
	})
}

func (s *Store) ListBlockedBy(issueID int) ([]Issue, error) {
	rows, err := s.readDB.Query(
		`SELECT i.id, i.title, i.body, i.state, i.kind, i.parent_issue_id, i.created_at, i.updated_at, i.closed_at
		 FROM issues i
		 JOIN issue_blocks ib ON i.id = ib.blocker_issue_id
		 WHERE ib.blocked_issue_id = ?
		 ORDER BY i.id`, issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list blocked by: %w", err)
	}
	defer rows.Close()
	issues := make([]Issue, 0)
	for rows.Next() {
		var i Issue
		if err := rows.Scan(&i.ID, &i.Title, &i.Body, &i.State, &i.Kind, &i.ParentIssueID, &i.CreatedAt, &i.UpdatedAt, &i.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan blocked by issue: %w", err)
		}
		issues = append(issues, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range issues {
		labels, err := s.getIssueLabels(s.readDB, issues[idx].ID)
		if err != nil {
			return nil, err
		}
		issues[idx].Labels = labels
	}
	return issues, nil
}

func (s *Store) ListBlocking(issueID int) ([]Issue, error) {
	rows, err := s.readDB.Query(
		`SELECT i.id, i.title, i.body, i.state, i.kind, i.parent_issue_id, i.created_at, i.updated_at, i.closed_at
		 FROM issues i
		 JOIN issue_blocks ib ON i.id = ib.blocked_issue_id
		 WHERE ib.blocker_issue_id = ?
		 ORDER BY i.id`, issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list blocking: %w", err)
	}
	defer rows.Close()
	issues := make([]Issue, 0)
	for rows.Next() {
		var i Issue
		if err := rows.Scan(&i.ID, &i.Title, &i.Body, &i.State, &i.Kind, &i.ParentIssueID, &i.CreatedAt, &i.UpdatedAt, &i.ClosedAt); err != nil {
			return nil, fmt.Errorf("scan blocking issue: %w", err)
		}
		issues = append(issues, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range issues {
		labels, err := s.getIssueLabels(s.readDB, issues[idx].ID)
		if err != nil {
			return nil, err
		}
		issues[idx].Labels = labels
	}
	return issues, nil
}

func (s *Store) CloseIssue(id int) error {
	state := "closed"
	return s.UpdateIssue(id, UpdateIssueOptions{State: &state})
}

func (s *Store) ReopenIssue(id int) error {
	state := "open"
	return s.UpdateIssue(id, UpdateIssueOptions{State: &state})
}

func (s *Store) AddComment(issueID int, body string) (*Comment, error) {
	return retryVal("AddComment", func() (*Comment, error) {
		if body == "" {
			return nil, fmt.Errorf("comment body is required")
		}
		if _, err := s.GetIssue(issueID); err != nil {
			return nil, err
		}
		result, err := s.writeDB.Exec(
			"INSERT INTO comments (issue_id, body) VALUES (?, ?)",
			issueID, body,
		)
		if err != nil {
			return nil, fmt.Errorf("add comment: %w", err)
		}
		id, _ := result.LastInsertId()
		return s.getComment(s.writeDB, int(id))
	})
}

func (s *Store) getComment(q Querier, id int) (*Comment, error) {
	var c Comment
	err := q.QueryRow(
		"SELECT id, issue_id, body, created_at FROM comments WHERE id = ?", id,
	).Scan(&c.ID, &c.IssueID, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", id, err)
	}
	return &c, nil
}

func (s *Store) ListComments(issueID int) ([]Comment, error) {
	if _, err := s.GetIssue(issueID); err != nil {
		return nil, err
	}
	rows, err := s.readDB.Query(
		"SELECT id, issue_id, body, created_at FROM comments WHERE issue_id = ? ORDER BY id", issueID,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.IssueID, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}
