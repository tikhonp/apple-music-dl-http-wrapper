package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DownloadRequest struct {
	URL     string `json:"url"`
	Format  string `json:"format,omitempty"`
	Song    bool   `json:"song,omitempty"`
	Debug   bool   `json:"debug,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // timeout in seconds, default 3600 (1 hour)
}

type DownloadStatus struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Status    string     `json:"status"`
	Progress  string     `json:"progress,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Logs      []string   `json:"logs,omitempty"`
	Duration  string     `json:"duration,omitempty"`
}

type JobManager struct {
	mu   sync.RWMutex
	jobs map[string]*DownloadStatus
}

func NewJobManager() *JobManager {
	return &JobManager{
		jobs: make(map[string]*DownloadStatus),
	}
}

func (jm *JobManager) CreateJob(url string) *DownloadStatus {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	id := uuid.New().String()
	job := &DownloadStatus{
		ID:        id,
		URL:       url,
		Status:    "pending",
		StartedAt: time.Now(),
		Logs:      []string{},
	}
	jm.jobs[id] = job
	return job
}

func (jm *JobManager) GetJob(id string) (*DownloadStatus, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, exists := jm.jobs[id]
	return job, exists
}

func (jm *JobManager) GetAllJobs() []*DownloadStatus {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	jobs := make([]*DownloadStatus, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (jm *JobManager) UpdateJob(id string, updater func(*DownloadStatus)) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, exists := jm.jobs[id]; exists {
		updater(job)
	}
}

func (jm *JobManager) AppendLog(id string, logLine string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if job, exists := jm.jobs[id]; exists {
		// Don't add empty lines
		if strings.TrimSpace(logLine) == "" {
			return
		}

		job.Logs = append(job.Logs, logLine)
		job.Progress = logLine

		// Keep only last 100 log lines to prevent memory issues
		if len(job.Logs) > 100 {
			job.Logs = job.Logs[len(job.Logs)-100:]
		}
	}
}

var jobManager = NewJobManager()

func main() {
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/status/", handleStatus)
	http.HandleFunc("/jobs", handleListJobs)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/cancel/", handleCancel)

	port := ":8080"
	log.Printf("Starting API server on %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Default timeout to 1 hour
	if req.Timeout == 0 {
		req.Timeout = 3600
	}

	// Create job
	job := jobManager.CreateJob(req.URL)

	// Start download in background
	go executeDownload(job.ID, req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"job_id": job.ID,
		"status": "started",
	})
}

// Custom split function that handles both \n and \r
func scanLinesOrCarriageReturn(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for \n or \r
	if i := bytes.IndexAny(data, "\n\r"); i >= 0 {
		// We have a full line
		return i + 1, data[0:i], nil
	}

	// If we're at EOF, return what we have
	if atEOF {
		return len(data), data, nil
	}

	// Request more data
	return 0, nil, nil
}

// Read output with proper handling of \r (carriage return) for progress updates
func readOutput(reader io.Reader, jobID string, prefix string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	// Use custom split function that handles both \n and \r
	scanner.Split(scanLinesOrCarriageReturn)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed != "" {
			log.Printf("[Job %s] %s: %s", jobID, prefix, trimmed)
			jobManager.AppendLog(jobID, trimmed)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[Job %s] Scanner error (%s): %v", jobID, prefix, err)
		jobManager.AppendLog(jobID, fmt.Sprintf("Scanner error: %v", err))
	}
}

func executeDownload(jobID string, req DownloadRequest) {
	startTime := time.Now()

	// Update status to running
	jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
		job.Status = "running"
	})
	jobManager.AppendLog(jobID, fmt.Sprintf("Starting download at %s", startTime.Format(time.RFC3339)))

	// Build command
	args := []string{}

	// Add format flags
	switch req.Format {
	case "atmos":
		args = append(args, "--atmos")
		jobManager.AppendLog(jobID, "Format: Dolby Atmos")
	case "aac":
		args = append(args, "--aac")
		jobManager.AppendLog(jobID, "Format: AAC")
	default:
		jobManager.AppendLog(jobID, "Format: ALAC (default)")
	}

	// Add song flag
	if req.Song {
		args = append(args, "--song")
		jobManager.AppendLog(jobID, "Mode: Single song")
	}

	// Add debug flag
	if req.Debug {
		args = append(args, "--debug")
		jobManager.AppendLog(jobID, "Debug mode enabled")
	}

	// Add URL
	args = append(args, req.URL)

	cmdStr := fmt.Sprintf("/usr/local/bin/apple-music-dl %v", args)
	jobManager.AppendLog(jobID, fmt.Sprintf("Command: %s", cmdStr))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	// Execute command with context
	cmd := exec.CommandContext(ctx, "/usr/local/bin/apple-music-dl", args...)

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		finishJobWithError(jobID, fmt.Errorf("failed to create stdout pipe: %w", err), startTime)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		finishJobWithError(jobID, fmt.Errorf("failed to create stderr pipe: %w", err), startTime)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		finishJobWithError(jobID, fmt.Errorf("failed to start command: %w", err), startTime)
		return
	}

	jobManager.AppendLog(jobID, fmt.Sprintf("Process started (PID: %d)", cmd.Process.Pid))

	// Read output in goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		readOutput(stdout, jobID, "STDOUT")
	}()

	go func() {
		defer wg.Done()
		readOutput(stderr, jobID, "STDERR")
	}()

	// Monitor progress
	done := make(chan error, 1)
	go func() {
		wg.Wait()
		done <- cmd.Wait()
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		duration := time.Since(startTime)
		now := time.Now()

		if ctx.Err() == context.DeadlineExceeded {
			jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
				job.Status = "failed"
				job.Error = fmt.Sprintf("Download timed out after %v", duration)
				job.EndedAt = &now
				job.Duration = duration.String()
			})
			log.Printf("[Job %s] Timed out after %v", jobID, duration)
		} else if err != nil {
			jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
				job.Status = "failed"
				job.Error = err.Error()
				job.EndedAt = &now
				job.Duration = duration.String()
			})
			log.Printf("[Job %s] Failed after %v: %v", jobID, duration, err)
		} else {
			jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
				job.Status = "completed"
				job.EndedAt = &now
				job.Duration = duration.String()
			})
			jobManager.AppendLog(jobID, "Download completed successfully!")
			log.Printf("[Job %s] Completed successfully in %v", jobID, duration)
		}
	}
}

func finishJobWithError(jobID string, err error, startTime time.Time) {
	now := time.Now()
	duration := time.Since(startTime)
	jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
		job.Status = "failed"
		job.Error = err.Error()
		job.EndedAt = &now
		job.Duration = duration.String()
	})
	log.Printf("[Job %s] Failed: %v", jobID, err)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Path[len("/status/"):]
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, exists := jobManager.GetJob(jobID)
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func handleListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs := jobManager.GetAllJobs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

func handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Path[len("/cancel/"):]
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, exists := jobManager.GetJob(jobID)
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	if job.Status != "running" {
		http.Error(w, "Job is not running", http.StatusBadRequest)
		return
	}

	// Note: This is a simplified cancel - in production you'd want to track
	// the actual process and kill it
	now := time.Now()
	jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
		job.Status = "cancelled"
		job.Error = "Cancelled by user"
		job.EndedAt = &now
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cancelled",
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
