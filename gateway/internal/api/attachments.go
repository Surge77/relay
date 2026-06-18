package api

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/model"
)

const maxAttachmentBytes = 10 << 20 // 10 MiB

// allowedAttachmentTypes is the content-type allowlist for uploads.
var allowedAttachmentTypes = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
}

type attachmentView struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

// handleUpload accepts a multipart file for a conversation the caller belongs to,
// stores the bytes, and records metadata. Returns the attachment + download URL.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.blob == nil {
		writeErr(w, http.StatusNotImplemented, "NO_STORAGE", "attachments are not configured")
		return
	}
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentBytes)
	if err := r.ParseMultipartForm(maxAttachmentBytes); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "TOO_LARGE", "file exceeds 10 MiB")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "a 'file' field is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !allowedAttachmentTypes[contentType] {
		writeErr(w, http.StatusUnsupportedMediaType, "BAD_TYPE", "unsupported content type")
		return
	}

	width, height := 0, 0
	if cfg, _, derr := image.DecodeConfig(file); derr == nil {
		width, height = cfg.Width, cfg.Height
	}
	if _, err := file.Seek(0, 0); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not read file")
		return
	}

	key := uuid.NewString()
	if err := s.blob.Put(key, file); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not store file")
		return
	}
	att := model.Attachment{
		ConversationID: convID, UploaderID: userIDFrom(r.Context()),
		Filename: header.Filename, ContentType: contentType, SizeBytes: header.Size,
		StorageKey: key, Width: width, Height: height,
	}
	id, err := s.store.InsertAttachment(r.Context(), att)
	if err != nil {
		_ = s.blob.Delete(key)
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not record attachment")
		return
	}
	writeJSON(w, http.StatusCreated, attachmentView{
		ID: id, URL: "/attachments/" + id, Filename: att.Filename,
		ContentType: att.ContentType, SizeBytes: att.SizeBytes, Width: width, Height: height,
	})
}

// handleDownload streams an attachment to a member of its conversation.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if s.blob == nil {
		writeErr(w, http.StatusNotImplemented, "NO_STORAGE", "attachments are not configured")
		return
	}
	att, err := s.store.AttachmentByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "attachment not found")
		return
	}
	if role, rerr := s.store.MemberRole(r.Context(), att.ConversationID, userIDFrom(r.Context())); rerr != nil || role == "" {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	rc, err := s.blob.Open(att.StorageKey)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "blob missing")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(att.SizeBytes, 10))
	w.Header().Set("Content-Disposition", "inline; filename=\""+att.Filename+"\"")
	_, _ = io.Copy(w, rc)
}
