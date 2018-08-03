package golang

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"os"

	"github.com/gaia-pipeline/protobuf"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// coreProtocolVersion is the protocol version of the plugin system itself.
const coreProtocolVersion = 1

// ProtocolVersion currently in use by Gaia
const ProtocolVersion = 2

// ProtocolType is the type used to communicate.
const ProtocolType = "grpc"

// List domain (usually localhost)
const listenIP = "localhost:"

// env variable key names for TLS cert path
const serverCertEnv = "GAIA_PLUGIN_CERT"
const serverKeyEnv = "GAIA_PLUGIN_KEY"
const rootCACertEnv = "GAIA_PLUGIN_CA_CERT"

var (
	// ErrorJobNotFound is returned when a given job id was not found
	// locally.
	ErrorJobNotFound = errors.New("job not found in plugin")

	// ErrorExitPipeline is used to safely exit the pipeline (actually not an error).
	// Prevents the pipeline to be marked as 'failed'.
	ErrorExitPipeline = errors.New("pipeline exit requested by job")

	// ErrorDuplicateJob is returned when two jobs have the same title which is restricted.
	ErrorDuplicateJob = errors.New("duplicate job found (two jobs with same title)")

	// errCertNotAppended is thrown when the root CA cert cannot be appended to the pool.
	errCertNotAppended = errors.New("cannot append root CA cert to cert pool")
)

// CachedJobs holds a list of JobsWrapper for later processing
var cachedJobs []jobsWrapper

// GRPCServer is the plugin gRPC implementation.
type GRPCServer struct{}

// GetJobs streams all given jobs back.
func (GRPCServer) GetJobs(empty *proto.Empty, stream proto.Plugin_GetJobsServer) error {
	for _, job := range cachedJobs {
		err := stream.Send(&job.job)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExecuteJob receives a job and executes it.
// Returns a JobResult object which gives information about job execution.
func (GRPCServer) ExecuteJob(ctx context.Context, j *proto.Job) (*proto.JobResult, error) {
	job := getJob((*j).UniqueId)
	if job == nil {
		return nil, ErrorJobNotFound
	}

	// Execute Job
	err := job.funcPointer(j.GetArgs())

	// Generate result object only when we got an error
	r := &proto.JobResult{}
	if err != nil {
		// Check if job wants to force exit pipeline.
		// We will exit the pipeline but not mark as 'failed'.
		if err == ErrorExitPipeline {
			r.ExitPipeline = true
		} else {
			// We got an error. Pipeline is now marked as 'failed'.
			r.ExitPipeline = true
			r.Failed = true
		}

		// Set log message and job id
		r.Message = err.Error()
		r.UniqueId = job.job.UniqueId
	}

	return r, err
}

// Serve initiates the gRPC Server and listens...forever.
// This method should be last called in the plugin main function.
func Serve(j Jobs) error {
	// Cache the jobs list for later processing
	// We first have to translate given jobs to different structure.
	cachedJobs = []jobsWrapper{}
	var priorityCounter int
	for _, job := range j {
		// Check for priority
		if job.Priority == 0 {
			priorityCounter++
		}

		// Create proto jobs object
		p := proto.Job{
			UniqueId:    hash(job.Title),
			Title:       job.Title,
			Description: job.Description,
			Priority:    job.Priority,
			Args:        job.Args,
		}

		// Create jobs wrapper object
		w := jobsWrapper{
			funcPointer: job.Handler,
			job:         p,
		}
		cachedJobs = append(cachedJobs, w)
	}

	// If all priorities are zero we set them auto
	if len(j) == priorityCounter {
		var priority int64
		for _, job := range cachedJobs {
			job.job.Priority = priority
			priority++
		}
	}

	// Check if two jobs have the same title which is restricted
	for x, job := range cachedJobs {
		for y, innerJob := range cachedJobs {
			if x != y && job.job.UniqueId == innerJob.job.UniqueId {
				return ErrorDuplicateJob
			}
		}
	}

	// Get certificates path from environment variables
	certPath := os.Getenv(serverCertEnv)
	keyPath := os.Getenv(serverKeyEnv)
	caCertPath := os.Getenv(rootCACertEnv)

	// Check if all certs are available
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return errors.Wrap(err, "cannot find path to certificate")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return errors.Wrap(err, "cannot find path to key")
	}
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		return errors.Wrap(err, "cannot find path to root CA certificate")
	}

	// implement health service
	health := health.NewServer()
	health.SetServingStatus("plugin", healthpb.HealthCheckResponse_SERVING)

	// Generate TLS config
	tlsConfig, err := generateTLSConfig(certPath, keyPath, caCertPath)
	if err != nil {
		return errors.Wrap(err, "cannot create TLS config")
	}

	// Create new gRPC server and register services
	s := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	proto.RegisterPluginServer(s, &GRPCServer{})
	healthpb.RegisterHealthServer(s, health)

	// Register reflection service on gRPC server
	reflection.Register(s)

	// Create TCP Server
	lis, err := net.Listen("tcp", listenIP)
	if err != nil {
		return errors.Wrap(err, "cannot start tcp server")
	}

	// Output the address and service name to stdout.
	// hashicorp go-plugin will use that to establish connection.
	fmt.Printf("%d|%d|%s|%s|%s\n",
		coreProtocolVersion,
		ProtocolVersion,
		lis.Addr().Network(),
		lis.Addr().String(),
		ProtocolType)
	os.Stdout.Sync()

	// Listen
	if err := s.Serve(lis); err != nil {
		return errors.Wrap(err, "cannot start grpc server")
	}
	return nil
}

// hash hashes the given string.
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
