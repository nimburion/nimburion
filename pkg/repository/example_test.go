package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/nimburion/nimburion/pkg/repository"
)

// User is an example entity
type User struct {
	ID      int64  `db:"id"`
	Name    string `db:"name"`
	Email   string `db:"email"`
	Version int64  `db:"version"`
}

func (u *User) GetVersion() int64 {
	return u.Version
}

func (u *User) SetVersion(version int64) {
	u.Version = version
}

// UserMapper implements EntityMapper for User
type UserMapper struct{}

func (m *UserMapper) ToRow(user *User) ([]string, []interface{}, error) {
	return []string{"id", "name", "email", "version"},
		[]interface{}{user.ID, user.Name, user.Email, user.Version},
		nil
}

func (m *UserMapper) FromRow(rows *sql.Rows) (*User, error) {
	user := &User{}
	err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Version)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (m *UserMapper) GetID(user *User) int64 {
	return user.ID
}

func (m *UserMapper) SetID(user *User, id int64) {
	user.ID = id
}

// Example demonstrates how to use the generic CRUD repository
func Example() {
	// This is a conceptual example - in real code, you would have a real database connection
	var db *sql.DB // Assume this is initialized

	// Create a repository for User entities
	userRepo := repository.NewGenericCrudRepository[User, int64](
		db,
		"users",
		"id",
		&UserMapper{},
	)

	ctx := context.Background()

	// Create a new user
	user := &User{
		ID:      1,
		Name:    "John Doe",
		Email:   "john@example.com",
		Version: 0,
	}
	if err := userRepo.Create(ctx, user); err != nil {
		log.Fatal(err)
	}

	// Find user by ID
	foundUser, err := userRepo.FindByID(ctx, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found user: %s\n", foundUser.Name)

	// Find all users with filters, sorting, and pagination
	opts := repository.QueryOptions{
		Filter: repository.Filter{
			"name": "John Doe",
		},
		Sort: repository.Sort{
			Field: "email",
			Order: repository.SortAsc,
		},
		Pagination: repository.Pagination{
			Page:     1,
			PageSize: 10,
		},
	}
	users, err := userRepo.FindAll(ctx, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d users\n", len(users))

	// Update user
	foundUser.Email = "john.doe@example.com"
	if err := userRepo.Update(ctx, foundUser); err != nil {
		log.Fatal(err)
	}

	// Count users
	count, err := userRepo.Count(ctx, repository.Filter{"name": "John Doe"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total users: %d\n", count)

	// Delete user
	if err := userRepo.Delete(ctx, 1); err != nil {
		log.Fatal(err)
	}
}
