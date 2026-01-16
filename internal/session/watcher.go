package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchEvent represents a session change event
type WatchEvent struct {
	Type     string         // "discovered", "updated", "new_commands"
	Session  *Session       // The affected session
	Commands []CommandEntry // New commands (for "new_commands" type)
}

// Watcher monitors the Claude projects directory for session changes
type Watcher struct {
	fsWatcher   *fsnotify.Watcher
	projectsDir string
	sessions    map[string]*Session // keyed by main session file path
	offsets     map[string]int64    // file read offsets for incremental parsing
	lineNumbers map[string]int      // line numbers for incremental parsing (1-indexed, next line to read)
	subagentMap map[string]string   // maps subagent file path -> main session file path
	mu          sync.RWMutex

	Events chan WatchEvent
	Errors chan error
	done   chan struct{}
}

// NewWatcher creates a new session watcher
func NewWatcher(projectsDir string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher:   fsw,
		projectsDir: projectsDir,
		sessions:    make(map[string]*Session),
		offsets:     make(map[string]int64),
		lineNumbers: make(map[string]int),
		subagentMap: make(map[string]string),
		Events:      make(chan WatchEvent, 100),
		Errors:      make(chan error, 10),
		done:        make(chan struct{}),
	}

	return w, nil
}

// DiscoverSessions scans for existing session files
func (w *Watcher) DiscoverSessions() ([]*Session, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var sessions []*Session

	entries, err := os.ReadDir(w.projectsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(w.projectsDir, entry.Name())

		// Find all JSONL files in this project directory
		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			session := w.parseSessionFile(jsonlPath, entry.Name())
			if session != nil {
				sessions = append(sessions, session)
				w.sessions[jsonlPath] = session

				// Track file size for incremental updates
				if info, err := os.Stat(jsonlPath); err == nil {
					w.offsets[jsonlPath] = info.Size()
				}

				// Watch and track subagent files
				sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
				subagentDir := filepath.Join(projectDir, sessionID, "subagents")
				if subagentFiles, err := filepath.Glob(filepath.Join(subagentDir, "*.jsonl")); err == nil {
					for _, subPath := range subagentFiles {
						w.subagentMap[subPath] = jsonlPath
						if info, err := os.Stat(subPath); err == nil {
							w.offsets[subPath] = info.Size()
						}
					}
					// Watch the subagent directory for new files
					if len(subagentFiles) > 0 {
						w.fsWatcher.Add(subagentDir)
					}
				}
			}
		}

		// Watch the project directory for new sessions
		if err := w.fsWatcher.Add(projectDir); err != nil {
			// Non-fatal: just skip this directory
			continue
		}
	}

	// Sort by last activity (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActivity.After(sessions[j].LastActivity)
	})

	return sessions, nil
}

// parseSessionFile creates a Session from a JSONL file
func (w *Watcher) parseSessionFile(path, encodedProject string) *Session {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	// Extract session ID from filename
	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")

	// Parse the main session file
	commands, meta, err := ParseSessionFile(path)
	if err != nil {
		return nil
	}

	// Use CWD from session file if available, otherwise show the encoded directory name
	projectPath := meta.CWD
	if projectPath == "" {
		projectPath = encodedProject
	}

	// Also parse subagent files if they exist
	subagentDir := filepath.Join(filepath.Dir(path), sessionID, "subagents")
	if subagentFiles, err := filepath.Glob(filepath.Join(subagentDir, "*.jsonl")); err == nil {
		for _, subagentPath := range subagentFiles {
			subCommands, _, _ := ParseSessionFile(subagentPath)
			commands = append(commands, subCommands...)
		}
	}

	// Sort all commands by timestamp
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Timestamp.Before(commands[j].Timestamp)
	})

	// Determine last activity time
	lastActivity := info.ModTime()
	if len(commands) > 0 {
		lastCmd := commands[len(commands)-1]
		if lastCmd.Timestamp.After(lastActivity) {
			lastActivity = lastCmd.Timestamp
		}
	}

	// Check subagent directory for recent modifications too
	if subagentInfo, err := os.Stat(subagentDir); err == nil {
		if subagentInfo.ModTime().After(info.ModTime()) {
			lastActivity = subagentInfo.ModTime()
		}
	}

	// Consider active if modified in last 5 minutes
	isActive := time.Since(lastActivity) < 5*time.Minute

	return &Session{
		ID:           sessionID,
		ProjectPath:  projectPath,
		FilePath:     path,
		GitBranch:    meta.GitBranch,
		LastActivity: lastActivity,
		Commands:     commands,
		IsActive:     isActive,
	}
}


// Start begins watching for file changes
func (w *Watcher) Start() {
	go w.watchLoop()
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// watchLoop handles fsnotify events
func (w *Watcher) watchLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleFSEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			select {
			case w.Errors <- err:
			default:
				// Error channel full, drop
			}
		}
	}
}

// handleFSEvent processes a filesystem event
func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	// Only care about JSONL files
	if !strings.HasSuffix(event.Name, ".jsonl") {
		return
	}

	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		w.handleFileUpdate(event.Name)

	case event.Op&fsnotify.Create == fsnotify.Create:
		w.handleNewFile(event.Name)
	}
}

// handleFileUpdate processes an updated session file
func (w *Watcher) handleFileUpdate(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if this is a subagent file
	mainSessionPath, isSubagent := w.subagentMap[path]
	var session *Session
	var exists bool

	if isSubagent {
		session, exists = w.sessions[mainSessionPath]
	} else {
		session, exists = w.sessions[path]
	}

	if !exists {
		return
	}

	// Get current offset and line number
	offset := w.offsets[path]
	startLine := w.lineNumbers[path]

	// Parse new content from offset
	newCommands, meta, newOffset, newLine, err := ParseSessionFileFrom(path, offset, startLine)
	if err != nil {
		return
	}

	// Update offset and line number
	w.offsets[path] = newOffset
	w.lineNumbers[path] = newLine

	// Update session metadata if we now have better info
	// This handles the case where the session was created before CWD was available
	if meta.CWD != "" && session.ProjectPath != meta.CWD {
		session.ProjectPath = meta.CWD
	}
	if meta.GitBranch != "" && session.GitBranch == "" {
		session.GitBranch = meta.GitBranch
	}

	if len(newCommands) == 0 {
		return
	}

	// Append new commands to session
	session.Commands = append(session.Commands, newCommands...)
	session.LastActivity = time.Now()
	session.IsActive = true

	// Send event
	select {
	case w.Events <- WatchEvent{
		Type:     "new_commands",
		Session:  session,
		Commands: newCommands,
	}:
	default:
		// Event channel full
	}
}

// handleNewFile processes a newly created session file
func (w *Watcher) handleNewFile(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if this is a new subagent file
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "subagents" {
		// This is a subagent file - find the parent session
		sessionDir := filepath.Dir(dir)
		sessionID := filepath.Base(sessionDir)
		projectDir := filepath.Dir(sessionDir)

		// Look for the main session file
		mainSessionPath := filepath.Join(projectDir, sessionID+".jsonl")
		if session, exists := w.sessions[mainSessionPath]; exists {
			// Track this subagent file
			w.subagentMap[path] = mainSessionPath
			if info, err := os.Stat(path); err == nil {
				w.offsets[path] = info.Size()
			}

			// Parse and add its commands to the session
			commands, _, _ := ParseSessionFile(path)
			if len(commands) > 0 {
				session.Commands = append(session.Commands, commands...)
				session.LastActivity = time.Now()
				session.IsActive = true

				// Send event
				select {
				case w.Events <- WatchEvent{
					Type:     "new_commands",
					Session:  session,
					Commands: commands,
				}:
				default:
				}
			}
		}
		return
	}

	// Regular session file
	// Already tracking this file?
	if _, exists := w.sessions[path]; exists {
		return
	}

	// Get the encoded project name from parent directory
	encodedProject := filepath.Base(filepath.Dir(path))

	session := w.parseSessionFile(path, encodedProject)
	if session == nil {
		return
	}

	w.sessions[path] = session

	// Track file size
	if info, err := os.Stat(path); err == nil {
		w.offsets[path] = info.Size()
	}

	// Send event
	select {
	case w.Events <- WatchEvent{
		Type:    "discovered",
		Session: session,
	}:
	default:
	}
}

// GetSessions returns a copy of all tracked sessions
func (w *Watcher) GetSessions() []*Session {
	w.mu.RLock()
	defer w.mu.RUnlock()

	sessions := make([]*Session, 0, len(w.sessions))
	for _, s := range w.sessions {
		sessions = append(sessions, s)
	}

	// Sort by last activity
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActivity.After(sessions[j].LastActivity)
	})

	return sessions
}

// RefreshActivityStatus updates IsActive flag for all sessions
func (w *Watcher) RefreshActivityStatus() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for path, session := range w.sessions {
		if info, err := os.Stat(path); err == nil {
			session.IsActive = time.Since(info.ModTime()) < 5*time.Minute
		}
	}
}
