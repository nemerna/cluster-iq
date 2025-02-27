// Package main implements the ClusterIQ Agent application.
//
// The ClusterIQ Agent is responsible for managing cloud provider accounts,
// initializing cloud executors, and exposing gRPC services for external interaction.
// It serves as a key component of the ClusterIQ system, enabling inventory management
// and operations on cloud resources.
//
// Features of the ClusterIQ Agent:
// - Manages configurations for multiple cloud providers.
// - Initializes and maintains executors for cloud provider accounts.
// - Provides a gRPC service interface for interacting with the ClusterIQ system.
// - Logs detailed information about operations for debugging and monitoring.
//
// The application uses gRPC as the communication protocol and supports extensible
// cloud executor implementations for AWS, GCP, and Azure.
package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc/reflection"

	pb "github.com/RHEcosystemAppEng/cluster-iq/generated/agent"

	cexec "github.com/RHEcosystemAppEng/cluster-iq/internal/cloud_executors"
	"github.com/RHEcosystemAppEng/cluster-iq/internal/config"
	"github.com/RHEcosystemAppEng/cluster-iq/internal/credentials"
	"github.com/RHEcosystemAppEng/cluster-iq/internal/inventory"

	ciqLogger "github.com/RHEcosystemAppEng/cluster-iq/internal/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

var (
	// logger is a shared logging instance used across the entire Agent application.
	logger *zap.Logger
	// version reflects the current version of the Agent.
	// It is populated at build time using build flags.
	version string
	// commit reflects the git short-hash of the compiled version.
	// It provides traceability for the exact source code version used to build the binary.
	commit string
)

// AgentService represents the main structure for managing cloud executors and configuration.
// It also embeds the gRPC server interface for handling gRPC requests.
type AgentService struct {
	cfg       *config.AgentConfig
	executors map[string]cexec.CloudExecutor
	logger    *zap.Logger
	pb.UnimplementedAgentServiceServer
}

// init initializes the global logger configuration.
// This is automatically invoked before the main function.
func init() {
	// Initialize logging configuration
	logger = ciqLogger.NewLogger()
}

// NewAgentService creates and initializes a new AgentService instance.
//
// Parameters:
//   - cfg: Pointer to AgentConfig containing the configuration details.
//   - logger: Pointer to zap.Logger for logging.
//
// Returns:
//   - *AgentService: A pointer to the newly created Agent instance.
func NewAgentService(cfg *config.AgentConfig, logger *zap.Logger) *AgentService {
	return &AgentService{
		cfg:       cfg,
		executors: make(map[string]cexec.CloudExecutor, 0),
		logger:    logger,
	}
}

// AddExecutor adds a new CloudExecutor to the AgentService.
//
// Parameters:
//   - exec: CloudExecutor instance to add.
//
// Returns:
//   - error: An error if the executor is nil; otherwise, nil.
func (a *AgentService) AddExecutor(exec cexec.CloudExecutor) error {
	if exec == nil {
		return fmt.Errorf("Cannot add a nil Executor")
	}

	a.executors[exec.GetAccountName()] = exec

	return nil
}

// readCloudProviderAccounts reads cloud provider account configurations from the credentials file.
//
// Returns:
//   - []credentials.AccountConfig: A slice of account configurations.
//   - error: An error if reading the file fails.
func (a *AgentService) readCloudProviderAccounts() ([]credentials.AccountConfig, error) {
	accounts, err := credentials.ReadCloudAccounts(a.cfg.Credentials.CredentialsFile)
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

// createExecutors initializes CloudExecutors for all configured cloud provider accounts.
//
// Returns:
//   - error: An error if any executor initialization fails.
func (a *AgentService) createExecutors() error {
	accounts, err := a.readCloudProviderAccounts()
	if err != nil {
		return err
	}

	// Generating a CloudExecutor by account. The creation of the CloudExecutor depends on the Cloud Provider
	for _, account := range accounts {
		switch account.Provider {
		case inventory.AWSProvider: // AWS
			a.logger.Info("Creating Executor for AWS account", zap.String("account_name", account.Name))
			exec := cexec.NewAWSExecutor(inventory.NewAccount("", account.Name, account.Provider, account.User, account.Key), logger)
			err := a.AddExecutor(exec)
			if err != nil {
				a.logger.Error("Cannot create an AWSEexecutor for account", zap.String("account_name", account.Name), zap.Error(err))
				return err
			}

		case inventory.GCPProvider: // GCP
			a.logger.Warn("Failed to create Executor for GCP account",
				zap.String("account", account.Name),
				zap.String("reason", "not implemented"),
			)

		case inventory.AzureProvider: // Azure
			a.logger.Warn("Failed to create Executor for Azure account",
				zap.String("account", account.Name),
				zap.String("reason", "not implemented"),
			)

		}
	}
	return nil
}

// LoggingInterceptor is a gRPC interceptor that logs information about incoming requests and their responses.
//
// It logs details such as the client's IP address, the invoked method, and any errors that occur during method execution.
// This interceptor can be used to enhance visibility and debugging in gRPC server operations.
//
// Parameters:
// - ctx: The context of the gRPC request.
// - req: The incoming request payload.
// - info: Metadata about the invoked gRPC method (e.g., method name).
// - handler: The actual handler function that processes the request.
//
// Returns:
// - An interface{} representing the response from the handler.
// - An error if the handler or any other operation fails.
func LoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		logger.Info("Client connected", zap.String("ip", p.Addr.String()))
	}

	log.Printf("Invoked method: %s", info.FullMethod)

	resp, err := handler(ctx, req)

	if err != nil {
		log.Printf("Error in method %s: %v", info.FullMethod, err)
	} else {
		log.Printf("Method %s executed successfully", info.FullMethod)
	}

	return resp, err
}

// main is the entry point for the ClusterIQ Agent application.
// It initializes the Agent, loads configuration, creates cloud executors, and starts the gRPC server.
func main() {
	// Ignore Logger sync error
	defer func() { _ = logger.Sync() }()

	var err error

	// Loading AgentService configuration
	cfg, err := config.LoadAgentConfig()
	if err != nil {
		logger.Fatal("Failed to load Agent config", zap.Error(err))
	}

	// Creating AgentService with the specified configuration
	agent := NewAgentService(cfg, logger)

	// Creating Executors
	err = agent.createExecutors()
	if err != nil {
		agent.logger.Panic("Error during CloudExecutors initialization", zap.Error(err))
	} else {
		agent.logger.Info("CloudExecutors initialization successfully", zap.Int("executors_count", len(agent.executors)))
	}

	// Initializing gRPC server
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(LoggingInterceptor))
	reflection.Register(grpcServer)

	// Registering Agent service on gRPC server
	pb.RegisterAgentServiceServer(grpcServer, agent)

	// Listener config
	lis, err := net.Listen("tcp", agent.cfg.ListenURL)
	if err != nil {
		logger.Error("Error initializing gRPC server on ClusterIQ Agent", zap.Error(err))
		return
	}
	logger.Info("gRPC ClusterIQ Agent initialization successfully",
		zap.String("listen_url", agent.cfg.ListenURL),
		zap.String("version", version),
		zap.String("commit", commit))
	// Serving gRPC
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
	logger.Info("ClusterIQ Agent Finished")
}
