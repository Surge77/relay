package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Surge77/relay/gateway/internal/model"
)

// InsertAttachment records uploaded-file metadata and returns the generated id.
func (s *Store) InsertAttachment(ctx context.Context, a model.Attachment) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO attachments (conversation_id, uploader_id, filename, content_type, size_bytes, storage_key, width, height)
		 VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,0),NULLIF($8,0)) RETURNING id`,
		a.ConversationID, a.UploaderID, a.Filename, a.ContentType, a.SizeBytes, a.StorageKey, a.Width, a.Height).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert attachment: %w", err)
	}
	return id, nil
}

// AttachmentByID fetches attachment metadata; ErrNotFound if absent.
func (s *Store) AttachmentByID(ctx context.Context, id string) (model.Attachment, error) {
	var a model.Attachment
	var w, h *int
	err := s.pool.QueryRow(ctx,
		`SELECT id, conversation_id, uploader_id, filename, content_type, size_bytes, storage_key, width, height
		   FROM attachments WHERE id=$1`, id).
		Scan(&a.ID, &a.ConversationID, &a.UploaderID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StorageKey, &w, &h)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Attachment{}, ErrNotFound
	}
	if err != nil {
		return model.Attachment{}, fmt.Errorf("attachment by id: %w", err)
	}
	if w != nil {
		a.Width = *w
	}
	if h != nil {
		a.Height = *h
	}
	return a, nil
}
