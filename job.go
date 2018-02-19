package golang

import (
	"github.com/gaia-pipeline/gaia/proto"
	uuid "github.com/satori/go.uuid"
)

// Jobs new type for wrapper around proto.job
type Jobs []JobsWrapper

// JobsWrapper wraps a function pointer around the
// proto.Job struct.
// The given function corresponds to the job.
type JobsWrapper struct {
	FuncPointer func() error
	Job         proto.Job
}

// Get looks up a job by the given id.
// Returns the job otherwise nil.
func (j *Jobs) Get(uniqueid string) *JobsWrapper {
	for _, job := range *j {
		if job.Job.UniqueId == uniqueid {
			return &job
		}
	}

	return nil
}

// SetUniqueID generates for every job a unique id.
// This function should be called once on plugin start.
// This is important for later execution.
func (j *Jobs) SetUniqueID() {
	for id := range *j {
		(*j)[id].Job.UniqueId = uuid.Must(uuid.NewV4()).String()
	}
}
