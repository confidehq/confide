package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phantompunk/confide/internal/db/queries"
)

var ErrNotFound = errors.New("identity key not found")

type DB interface {
	UpsertIdentityKey(ctx context.Context, arg queries.UpsertIdentityKeyParams) error
	GetIdentityKey(ctx context.Context, accountID string) (queries.GetIdentityKeyRow, error)
	GetIdentityPublicKey(ctx context.Context, accountID string) ([]byte, error)
}

type Service struct {
	db DB
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{db: queries.New(pool)}
}

type KeyPair struct {
	PublicKey          []byte
	WrappedPrivateKey  []byte
}

func (s *Service) Upsert(ctx context.Context, accountID string, publicKey, wrappedPrivateKey []byte) error {
	return s.db.UpsertIdentityKey(ctx, queries.UpsertIdentityKeyParams{
		AccountID:                accountID,
		IdentityPublicKey:        publicKey,
		WrappedIdentityPrivateKey: wrappedPrivateKey,
	})
}

// Get returns both keys for the caller's own account.
func (s *Service) Get(ctx context.Context, accountID string) (KeyPair, error) {
	row, err := s.db.GetIdentityKey(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KeyPair{}, ErrNotFound
		}
		return KeyPair{}, err
	}
	return KeyPair{
		PublicKey:         row.IdentityPublicKey,
		WrappedPrivateKey: row.WrappedIdentityPrivateKey,
	}, nil
}

// GetPublicKey returns only the public key for any account (safe to expose to other members).
func (s *Service) GetPublicKey(ctx context.Context, accountID string) ([]byte, error) {
	pub, err := s.db.GetIdentityPublicKey(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return pub, nil
}
