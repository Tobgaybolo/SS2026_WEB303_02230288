package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("purchase not found")

type purchaseRecord struct {
	ID             string
	PlantID        string
	PlantName      string
	UnitPriceCents int64
	Quantity       int32
	TotalCents     int64
	CustomerName   string
	Status         string
	CreatedAt      time.Time
}

// newPurchase carries everything needed to insert a purchase.
type newPurchase struct {
	PlantID        string
	PlantName      string
	UnitPriceCents int64
	Quantity       int32
	CustomerName   string
}

// plant_id deliberately has NO foreign key: plants live in Catalog's database,
// and this service may only reach them through Catalog's gRPC API
// (Database per Service). plant_name and unit_price_cents are snapshots, so
// purchase history stays correct even if the plant is later changed or deleted.
const schema = `
CREATE TABLE IF NOT EXISTS purchases (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plant_id         UUID        NOT NULL,
    plant_name       TEXT        NOT NULL,
    unit_price_cents BIGINT      NOT NULL CHECK (unit_price_cents > 0),
    quantity         INTEGER     NOT NULL CHECK (quantity BETWEEN 1 AND 100),
    total_cents      BIGINT      NOT NULL CHECK (total_cents > 0),
    customer_name    TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'COMPLETED', 'CANCELLED')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);`

const purchaseColumns = `id::text, plant_id::text, plant_name, unit_price_cents,
	quantity, total_cents, customer_name, status, created_at`

// Repository is the only place in this service that touches SQL.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Migrate(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, schema)
	return err
}

// scanPurchase works for both a single row and a row from a result set.
func scanPurchase(row pgx.Row) (*purchaseRecord, error) {
	var p purchaseRecord
	err := row.Scan(&p.ID, &p.PlantID, &p.PlantName, &p.UnitPriceCents, &p.Quantity,
		&p.TotalCents, &p.CustomerName, &p.Status, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) Create(ctx context.Context, np newPurchase) (*purchaseRecord, error) {
	total := np.UnitPriceCents * int64(np.Quantity)
	return scanPurchase(r.pool.QueryRow(ctx,
		`INSERT INTO purchases (plant_id, plant_name, unit_price_cents, quantity, total_cents, customer_name)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+purchaseColumns,
		np.PlantID, np.PlantName, np.UnitPriceCents, np.Quantity, total, np.CustomerName))
}

func (r *Repository) Get(ctx context.Context, id string) (*purchaseRecord, error) {
	return scanPurchase(r.pool.QueryRow(ctx,
		`SELECT `+purchaseColumns+` FROM purchases WHERE id = $1`, id))
}

func (r *Repository) List(ctx context.Context) ([]*purchaseRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+purchaseColumns+` FROM purchases ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*purchaseRecord
	for rows.Next() {
		p, err := scanPurchase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) Update(ctx context.Context, id string, quantity int32, totalCents int64,
	customerName, status string) (*purchaseRecord, error) {
	return scanPurchase(r.pool.QueryRow(ctx,
		`UPDATE purchases SET quantity = $2, total_cents = $3, customer_name = $4, status = $5
		 WHERE id = $1 RETURNING `+purchaseColumns,
		id, quantity, totalCents, customerName, status))
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM purchases WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}