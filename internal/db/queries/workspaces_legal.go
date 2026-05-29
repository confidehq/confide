package queries

import "context"

const getWorkspaceLegalText = `SELECT legal_text FROM workspaces WHERE id = $1`

func (q *Queries) GetWorkspaceLegalText(ctx context.Context, id string) (string, error) {
	row := q.db.QueryRow(ctx, getWorkspaceLegalText, id)
	var legalText string
	err := row.Scan(&legalText)
	return legalText, err
}

const updateWorkspaceLegalText = `UPDATE workspaces SET legal_text = $2, updated_at = NOW() WHERE id = $1`

func (q *Queries) UpdateWorkspaceLegalText(ctx context.Context, id, legalText string) error {
	_, err := q.db.Exec(ctx, updateWorkspaceLegalText, id, legalText)
	return err
}
