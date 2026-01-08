package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// UserRepo implements UserRepository using BadgerDB
type UserRepo struct {
	db *DB
}

// storageUser is an internal struct to ensure private fields are saved to DB
// despite json:"-" tags on domain.User
type storageUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	PublicKey    string    `json:"public_key"`
	PrivateKey   string    `json:"private_key"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toStorageUser(u *domain.User) *storageUser {
	return &storageUser{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		PublicKey:    u.PublicKey,
		PrivateKey:   u.PrivateKey,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func toDomainUser(s *storageUser) *domain.User {
	return &domain.User{
		ID:           s.ID,
		Username:     s.Username,
		Email:        s.Email,
		PasswordHash: s.PasswordHash,
		PublicKey:    s.PublicKey,
		PrivateKey:   s.PrivateKey,
		IsActive:     s.IsActive,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// NewUserRepo creates a new BadgerDB-based user repository
func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create creates a new user
func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Check username uniqueness
		usernameKey := []byte(fmt.Sprintf("user:username:%s", strings.ToLower(user.Username)))
		if _, err := txn.Get(usernameKey); err == nil {
			return domain.ErrUserAlreadyExists
		}

		// Check email uniqueness (if provided)
		if user.Email != "" {
			emailKey := []byte(fmt.Sprintf("user:email:%s", strings.ToLower(user.Email)))
			if _, err := txn.Get(emailKey); err == nil {
				return domain.ErrUserAlreadyExists
			}
		}

		// Save user data using storage struct
		data, err := json.Marshal(toStorageUser(user))
		if err != nil {
			return err
		}

		idKey := []byte(fmt.Sprintf("user:id:%s", user.ID))
		if err := txn.Set(idKey, data); err != nil {
			return err
		}

		// Set indexes
		if err := txn.Set(usernameKey, []byte(user.ID)); err != nil {
			return err
		}
		if user.Email != "" {
			emailKey := []byte(fmt.Sprintf("user:email:%s", strings.ToLower(user.Email)))
			if err := txn.Set(emailKey, []byte(user.ID)); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetByID retrieves a user by ID
func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var sUser storageUser
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("user:id:%s", id)))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrUserNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &sUser)
		})
	})
	if err != nil {
		return nil, err
	}
	return toDomainUser(&sUser), nil
}

// GetByUsername retrieves a user by username
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var id []byte
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("user:username:%s", strings.ToLower(username))))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrUserNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			id = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, string(id))
}

// GetByEmail retrieves a user by email
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if email == "" {
		return nil, domain.ErrUserNotFound
	}
	var id []byte
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("user:email:%s", strings.ToLower(email))))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrUserNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			id = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, string(id))
}

// Update updates an existing user
func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Get existing user to handle index updates if needed
		// For simplicity, assuming ID is immutable

		data, err := json.Marshal(toStorageUser(user))
		if err != nil {
			return err
		}
		return txn.Set([]byte(fmt.Sprintf("user:id:%s", user.ID)), data)
	})
}

// Delete deletes a user by ID
func (r *UserRepo) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Get user first to clean up indexes
		item, err := txn.Get([]byte(fmt.Sprintf("user:id:%s", id)))
		if err != nil {
			return err
		}
		
		var sUser storageUser
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &sUser)
		}); err != nil {
			return err
		}

		// Delete indexes
		if err := txn.Delete([]byte(fmt.Sprintf("user:username:%s", strings.ToLower(sUser.Username)))); err != nil {
			return err
		}
		if sUser.Email != "" {
			if err := txn.Delete([]byte(fmt.Sprintf("user:email:%s", strings.ToLower(sUser.Email)))); err != nil {
				return err
			}
		}

		// Delete data
		return txn.Delete([]byte(fmt.Sprintf("user:id:%s", id)))
	})
}

// ExistsByUsername checks if a user exists by username
func (r *UserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	err := r.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(fmt.Sprintf("user:username:%s", strings.ToLower(username))))
		return err
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return false, err
}

// ExistsByEmail checks if a user exists by email
func (r *UserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	err := r.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(fmt.Sprintf("user:email:%s", strings.ToLower(email))))
		return err
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return false, err
}
