package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/JackyTaan/grpc-redis-postgres/pkg/db"
	"github.com/JackyTaan/grpc-redis-postgres/pkg/redis"
	"github.com/JackyTaan/grpc-redis-postgres/proto"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env file: ", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Configure Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		log.Fatalf("Error parsing REDIS_DB: %v", err)
	}
	redisClient, err := redis.NewRedisClient(redisAddr, redisPassword, redisDB)
	if err != nil {
		log.Fatalf("Error creating Redis client: %v", err)
	}

	// Configure PostgreSQL
	dbDSN := os.Getenv("DB_DSN")
	dbClient, err := db.NewDatabaseClient(dbDSN)
	if err != nil {
		log.Fatalf("Error creating PostgreSQL client: %v", err)
	}

	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			zap.UnaryServerInterceptor(logger),
			validator.UnaryServerInterceptor(),
			recovery.UnaryServerInterceptor(),
			tags.UnaryServerInterceptor(),
		),
	)

	// Register User service
	proto.RegisterUserServiceServer(server, &userService{
		redisClient: redisClient,
		dbClient:    dbClient,
	})

	// Listen on port
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Server listening on port: %s", ":50051")

	// Serve gRPC requests
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// userService implements the proto.UserServiceServer interface
type userService struct {
	proto.UnimplementedUserServiceServer
	redisClient redis.RedisClient
	dbClient    db.DatabaseClient
}

// GetUser retrieves a user from Redis or PostgreSQL.
func (s *userService) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.User, error) {
	// 1. Try to retrieve user from Redis
	user, err := s.redisClient.GetUser(ctx, req.Id)
	if err == nil {
		return user, nil
	}

	// 2. If not found in Redis, retrieve from PostgreSQL
	user, err = s.dbClient.GetUser(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user with ID %s not found", req.Id)
	}

	// 3. Cache user in Redis
	if err := s.redisClient.SetUser(ctx, user); err != nil {
		// Error caching, but still return user
		log.Printf("Error caching user in Redis: %v", err)
	}

	return user, nil
}

// CreateUser creates a new user and stores it in Redis and PostgreSQL.
func (s *userService) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.User, error) {
	// 1. Create user in PostgreSQL
	user, err := s.dbClient.CreateUser(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error creating user: %v", err)
	}

	// 2. Cache user in Redis
	if err := s.redisClient.SetUser(ctx, user); err != nil {
		return nil, status.Errorf(codes.Internal, "error caching user in Redis: %v", err)
	}

	return user, nil
}
