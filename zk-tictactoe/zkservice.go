// Package zktictactoe provides a ZK-enabled variant of the tic-tac-toe service.
package zktictactoe

import (
	"log"
	"net/http"

	"github.com/pflow-xyz/petri-pilot/generated/tictactoe"
	"github.com/pflow-xyz/petri-pilot/pkg/serve"
)

// ZKServiceName is the registered name for this service.
const ZKServiceName = "zk-tic-tac-toe"

func init() {
	serve.Register(ZKServiceName, NewZKService)
}

// ZKService wraps the base tic-tac-toe service with ZK proof capabilities.
type ZKService struct {
	base *tictactoe.Service
	zk   *ZKIntegration
}

// NewZKService creates a new ZK-enabled tic-tac-toe service.
func NewZKService() (serve.Service, error) {
	// Create base service
	base, err := tictactoe.NewService()
	if err != nil {
		return nil, err
	}

	// Create ZK integration
	log.Println("Initializing ZK circuits...")
	zk, err := NewZKIntegration()
	if err != nil {
		base.Close()
		return nil, err
	}

	return &ZKService{
		base: base.(*tictactoe.Service),
		zk:   zk,
	}, nil
}

// Name returns the service name.
func (s *ZKService) Name() string {
	return ZKServiceName
}

// BuildHandler returns the HTTP handler with both base routes and ZK endpoints.
func (s *ZKService) BuildHandler() http.Handler {
	mux := http.NewServeMux()

	// Mount base handler at root
	baseHandler := s.base.BuildHandler()
	mux.Handle("/", baseHandler)

	// Mount ZK endpoints at /zk/
	mux.Handle("/zk/", http.StripPrefix("/zk", s.zk.Handler()))

	log.Println("  ZK endpoints mounted at /zk/")

	return mux
}

// Close cleans up resources.
func (s *ZKService) Close() error {
	return s.base.Close()
}

// GraphQLSchema returns the combined GraphQL schema for this service.
func (s *ZKService) GraphQLSchema() string {
	// Combine base schema with ZK schema
	return s.base.GraphQLSchema() + "\n" + ZKGraphQLSchema
}

// GraphQLResolvers returns the combined GraphQL resolvers for this service.
func (s *ZKService) GraphQLResolvers() map[string]serve.GraphQLResolver {
	// Start with base resolvers
	resolvers := s.base.GraphQLResolvers()

	// Add ZK resolvers
	for name, resolver := range s.zk.ZKGraphQLResolvers() {
		resolvers[name] = resolver
	}

	return resolvers
}
