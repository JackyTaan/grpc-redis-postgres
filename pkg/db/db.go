package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JackyTaan/grpc-redis-postgres/proto"
	_ "github.com/lib/pq"
)

type DatabaseClient interface {
	GetUser(ctx context.Context, id string) (*proto.User, error)
	CreateUser(ctx context.Context, user *proto.CreateUserRequest) (*proto.User, error)
}

type databaseClient struct {
	db *sql.DB
}

// NewDatabaseClient creates a new database client.
func NewDatabaseClient(dsn string) (DatabaseClient, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting to PostgreSQL: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging PostgreSQL: %w", err)
	}
	return &databaseClient{db: db}, nil
}

// GetUser retrieves a user from the database.
func (c *databaseClient) GetUser(ctx context.Context, id string) (*proto.User, error) {
	var user proto.User
	err := c.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = $1", id).
		Scan(&user.Id, &user.Name, &user.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user with ID %s not found", id)
		}
		return nil, fmt.Errorf("error retrieving user: %w", err)
	}
	return &user, nil
}

// CreateUser creates a new user in the database.
func (c *databaseClient) CreateUser(ctx context.Context, user *proto.CreateUserRequest) (*proto.User, error) {
	var userID string
	err := c.db.QueryRowContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id", user.Name, user.Email).
		Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}
	return &proto.User{
		Id:    userID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}
