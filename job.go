package golang

import (
	"hash/fnv"

	"github.com/gaia-pipeline/gaia/proto"
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
func (j *Jobs) Get(hash uint32) *JobsWrapper {
	for _, job := range *j {
		if job.Job.UniqueId == hash {
			return &job
		}
	}

	return nil
}

// SetUniqueID generates a hash of the title for every job.
// The output should be the same as long as the plugin does not change.
func (j *Jobs) SetUniqueID() {
	for id, job := range *j {
		(*j)[id].Job.UniqueId = hash(job.Job.Title)
	}
}

// hash hashes the given string.
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
