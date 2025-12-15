package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DownloadRequest struct {
	URL    string `json:"url"`
	Format string `json:"format,omitempty"` // "alac" (default), "atmos", "aac"
	Song   bool   `json:"song,omitempty"`
	Debug  bool   `json:"debug,omitempty"`
}

type DownloadStatus struct {
	ID        string     `json:"id"`
	URL       string     `json:"url"`
	Status    string     `json:"status"` // "pending", "running", "completed", "failed"
	Progress  string     `json:"progress,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Logs      []string   `json:"logs,omitempty"`
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
		job.Logs = append(job.Logs, logLine)
		job.Progress = logLine
	}
}

var jobManager = NewJobManager()

func main() {
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/status/", handleStatus)
	http.HandleFunc("/jobs", handleListJobs)
	http.HandleFunc("/health", handleHealth)

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

func executeDownload(jobID string, req DownloadRequest) {
	// Update status to running
	jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
		job.Status = "running"
	})

	// Build command
	args := []string{}

	// Add format flags
	switch req.Format {
	case "atmos":
		args = append(args, "--atmos")
	case "aac":
		args = append(args, "--aac")
	}

	// Add song flag
	if req.Song {
		args = append(args, "--song")
	}

	// Add debug flag
	if req.Debug {
		args = append(args, "--debug")
	}

	// Add URL
	args = append(args, req.URL)

	// Execute command
	cmd := exec.Command("/usr/local/bin/apple-music-dl", args...)

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		finishJobWithError(jobID, err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		finishJobWithError(jobID, err)
		return
	}

	// Start command
	if err := cmd.Start(); err != nil {
		finishJobWithError(jobID, err)
		return
	}

	// Read output in goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[Job %s] %s", jobID, line)
			jobManager.AppendLog(jobID, line)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[Job %s] ERROR: %s", jobID, line)
			jobManager.AppendLog(jobID, "ERROR: "+line)
		}
	}()

	// Wait for output to finish
	wg.Wait()

	// Wait for command to finish
	err = cmd.Wait()

	now := time.Now()
	if err != nil {
		jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
			job.Status = "failed"
			job.Error = err.Error()
			job.EndedAt = &now
		})
		log.Printf("[Job %s] Failed: %v", jobID, err)
	} else {
		jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
			job.Status = "completed"
			job.EndedAt = &now
		})
		log.Printf("[Job %s] Completed successfully", jobID)
	}
}

func finishJobWithError(jobID string, err error) {
	now := time.Now()
	jobManager.UpdateJob(jobID, func(job *DownloadStatus) {
		job.Status = "failed"
		job.Error = err.Error()
		job.EndedAt = &now
	})
	log.Printf("[Job %s] Failed: %v", jobID, err)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}
