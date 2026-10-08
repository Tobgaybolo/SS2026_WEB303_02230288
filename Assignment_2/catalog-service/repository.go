package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errNotFound is returned when no row matches; the handler maps it to NOT_FOUND.
var errNotFound = errors.New("plant not found")

// plantRecord is the database representation of a plant.
type plantRecord struct {
	ID         string
	Name       string
	Species    string
	PriceCents int64
	Stock      int32
	CreatedAt  time.Time
}

// schema is applied at startup. CHECK constraints are a second line of defence
// behind the validation layer. id is cast to text so it scans into a Go string.
const schema = `
CREATE TABLE IF NOT EXISTS plants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    species     TEXT        NOT NULL,
    price_cents BIGINT      NOT NULL CHECK (price_cents > 0),
    stock       INTEGER     NOT NULL CHECK (stock >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const plantColumns = `id::text, name, species, price_cents, stock, created_at`

// Repository is the only place in this service that touches SQL.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Migrate creates the table if it does not exist yet.
func (r *Repository) Migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, schema)
	return err
}

func scanPlant(row pgx.Row) (*plantRecord, error) {
	var p plantRecord
	err := row.Scan(&p.ID, &p.Name, &p.Species, &p.PriceCents, &p.Stock, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Create(ctx context.Context, in plantInput) (*plantRecord, error) {
	return scanPlant(r.pool.QueryRow(ctx,
		`INSERT INTO plants (name, species, price_cents, stock)
		 VALUES ($1, $2, $3, $4) RETURNING `+plantColumns,
		in.Name, in.Species, in.PriceCents, in.Stock))
}

func (r *Repository) Get(ctx context.Context, id string) (*plantRecord, error) {
	return scanPlant(r.pool.QueryRow(ctx,
		`SELECT `+plantColumns+` FROM plants WHERE id = $1`, id))
}

func (r *Repository) List(ctx context.Context) ([]*plantRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+plantColumns+` FROM plants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*plantRecord
	for rows.Next() {
		var p plantRecord
		if err := rows.Scan(&p.ID, &p.Name, &p.Species, &p.PriceCents, &p.Stock, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// Update replaces all editable fields (PUT semantics).
func (r *Repository) Update(ctx context.Context, id string, in plantInput) (*plantRecord, error) {
	return scanPlant(r.pool.QueryRow(ctx,
		`UPDATE plants SET name = $2, species = $3, price_cents = $4, stock = $5
		 WHERE id = $1 RETURNING `+plantColumns,
		id, in.Name, in.Species, in.PriceCents, in.Stock))
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM plants WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}