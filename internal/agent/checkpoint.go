package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

// StageCheckpoint records the state of a completed stage.
type StageCheckpoint struct {
	Name     string            `json:"name"`
	Complete bool              `json:"complete"`
	Outputs  map[string]string `json:"outputs"` // role -> output
}

// JobCheckpoint is the serializable state of a job that can be
// saved to disk and loaded to resume execution.
type JobCheckpoint struct {
	JobID     string            `json:"job_id"`
	Request   string            `json:"request"`
	Stages    []StageCheckpoint `json:"stages"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// jobsDir returns the directory where job checkpoints are stored.
func jobsDir() string {
	return filepath.Join(config.ConfigDir(), "jobs")
}

// checkpointPath returns the file path for a given job ID.
func checkpointPath(jobID string) string {
	return filepath.Join(jobsDir(), jobID+".json")
}

// SaveCheckpoint writes a checkpoint to disk as JSON atomically using a
// temp file + rename to prevent corruption from crashes during write.
func SaveCheckpoint(jobID string, checkpoint *JobCheckpoint) error {
	dir := jobsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating jobs dir: %w", err)
	}

	checkpoint.UpdatedAt = time.Now()
	if checkpoint.CreatedAt.IsZero() {
		checkpoint.CreatedAt = checkpoint.UpdatedAt
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling checkpoint: %w", err)
	}

	finalPath := checkpointPath(jobID)

	tmp, err := os.CreateTemp(dir, ".checkpoint-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp checkpoint file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp checkpoint file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp checkpoint file: %w", err)
	}

	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp checkpoint file: %w", err)
	}

	return nil
}

// LoadCheckpoint reads a checkpoint from disk.
func LoadCheckpoint(jobID string) (*JobCheckpoint, error) {
	path := checkpointPath(jobID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading checkpoint %q: %w", jobID, err)
	}

	var cp JobCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parsing checkpoint: %w", err)
	}

	return &cp, nil
}

// jobResultToCheckpoint converts the current JobResult into a
// checkpoint suitable for saving.
func jobResultToCheckpoint(result *JobResult) *JobCheckpoint {
	cp := &JobCheckpoint{
		JobID:   result.JobID,
		Request: result.Request,
	}

	for _, sr := range result.Stages {
		scp := StageCheckpoint{
			Name:     sr.StageName,
			Complete: true,
			Outputs:  make(map[string]string),
		}
		for role, output := range sr.WorkerOutputs {
			scp.Outputs[string(role)] = output
		}
		cp.Stages = append(cp.Stages, scp)
	}

	return cp
}
