package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	apperrors "unwise-backend/errors"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handlers) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	user, err := h.userService.GetUser(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (h *Handlers) UploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("[UploadUserAvatar] Failed to parse multipart form: %v", err)
		handleError(w, apperrors.InvalidRequest("Failed to parse multipart form. Maximum file size is 10MB."))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		log.Printf("[UploadUserAvatar] Failed to get avatar file: %v", err)
		handleError(w, apperrors.MissingRequiredField("Avatar image"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" && contentType != "image/gif" {
		handleError(w, apperrors.InvalidRequest("Invalid image format. Supported formats: JPEG, PNG, WebP, GIF."))
		return
	}

	filename := "user_" + userID + "_" + uuid.New().String() + "_" + time.Now().Format("20060102_150405")

	avatarURL, err := h.storageService.Upload(r.Context(), h.userAvatarsBucket, filename, file, contentType)
	if err != nil {
		log.Printf("[UploadUserAvatar] Failed to upload avatar: %v", err)
		handleError(w, apperrors.StorageError("uploading avatar", err))
		return
	}

	user, err := h.userService.UpdateAvatar(r.Context(), userID, avatarURL)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (h *Handlers) UploadGroupAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if groupID == "" {
		handleError(w, apperrors.MissingRequiredField("Group ID"))
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("[UploadGroupAvatar] Failed to parse multipart form: %v", err)
		handleError(w, apperrors.InvalidRequest("Failed to parse multipart form. Maximum file size is 10MB."))
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		log.Printf("[UploadGroupAvatar] Failed to get avatar file: %v", err)
		handleError(w, apperrors.MissingRequiredField("Avatar image"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" && contentType != "image/gif" {
		handleError(w, apperrors.InvalidRequest("Invalid image format. Supported formats: JPEG, PNG, WebP, GIF."))
		return
	}

	filename := "group_" + groupID + "_" + uuid.New().String() + "_" + time.Now().Format("20060102_150405")

	avatarURL, err := h.storageService.Upload(r.Context(), h.groupPhotosBucket, filename, file, contentType)
	if err != nil {
		log.Printf("[UploadGroupAvatar] Failed to upload avatar: %v", err)
		handleError(w, apperrors.StorageError("uploading group avatar", err))
		return
	}

	group, err := h.groupService.UpdateGroupAvatar(r.Context(), groupID, userID, avatarURL)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, group)
}

func (h *Handlers) GetClaimablePlaceholders(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	placeholders, err := h.userService.GetClaimablePlaceholders(r.Context(), userID)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, placeholders)
}

func (h *Handlers) ClaimPlaceholder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	placeholderID := chi.URLParam(r, "placeholderID")
	if _, err := uuid.Parse(placeholderID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Placeholder ID format."))
		return
	}

	if err := h.userService.ClaimPlaceholder(r.Context(), userID, placeholderID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Placeholder claimed successfully. All expenses have been transferred to your account.",
	})
}

type AssignPlaceholderRequest struct {
	UserID string `json:"user_id"`
}

func (h *Handlers) AssignPlaceholder(w http.ResponseWriter, r *http.Request) {
	placeholderID := chi.URLParam(r, "placeholderID")
	if _, err := uuid.Parse(placeholderID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Placeholder ID format."))
		return
	}

	var req AssignPlaceholderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid JSON"))
		return
	}

	if req.UserID == "" {
		handleError(w, apperrors.MissingRequiredField("user_id"))
		return
	}

	if _, err := uuid.Parse(req.UserID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid User ID format."))
		return
	}

	if err := h.userService.AssignPlaceholder(r.Context(), placeholderID, req.UserID); err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Placeholder assigned successfully.",
	})
}
