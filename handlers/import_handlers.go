package handlers

import (
	"encoding/json"
	"net/http"

	apperrors "unwise-backend/errors"
	"unwise-backend/services"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const maxUploadSize = 5 * 1024 * 1024

type ImportHandlers struct {
	importService services.ImportService
}

func NewImportHandlers(importService services.ImportService) *ImportHandlers {
	return &ImportHandlers{
		importService: importService,
	}
}

func (h *ImportHandlers) RegisterRoutes(r chi.Router) {
	r.Route("/groups/{groupID}/import", func(r chi.Router) {
		r.Post("/splitwise/preview", h.PreviewSplitwiseCSV)
		r.Post("/splitwise", h.ImportSplitwiseCSV)
	})
}

func (h *ImportHandlers) PreviewSplitwiseCSV(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		handleError(w, apperrors.InvalidRequest("File too large or invalid multipart form. Max size is 5MB."))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		handleError(w, apperrors.MissingRequiredField("file"))
		return
	}
	defer file.Close()

	if header.Header.Get("Content-Type") != "text/csv" &&
		!isCSVFilename(header.Filename) {
		handleError(w, apperrors.InvalidRequest("File must be a CSV file."))
		return
	}

	zap.L().Info("Previewing Splitwise CSV",
		zap.String("group_id", groupID),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size))

	result, err := h.importService.PreviewSplitwiseCSV(r.Context(), groupID, userID, file)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *ImportHandlers) ImportSplitwiseCSV(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		handleError(w, err)
		return
	}

	groupID := chi.URLParam(r, "groupID")
	if _, err := uuid.Parse(groupID); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid Group ID format."))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		handleError(w, apperrors.InvalidRequest("File too large or invalid multipart form. Max size is 5MB."))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		handleError(w, apperrors.MissingRequiredField("file"))
		return
	}
	defer file.Close()

	if header.Header.Get("Content-Type") != "text/csv" &&
		!isCSVFilename(header.Filename) {
		handleError(w, apperrors.InvalidRequest("File must be a CSV file."))
		return
	}

	mappingJSON := r.FormValue("member_mapping")
	if mappingJSON == "" {
		handleError(w, apperrors.MissingRequiredField("member_mapping"))
		return
	}

	var memberMapping map[string]*string
	if err := json.Unmarshal([]byte(mappingJSON), &memberMapping); err != nil {
		handleError(w, apperrors.InvalidRequest("Invalid member_mapping JSON format."))
		return
	}

	zap.L().Info("Importing Splitwise CSV",
		zap.String("group_id", groupID),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size),
		zap.Int("mappings", len(memberMapping)))

	result, err := h.importService.ImportSplitwiseCSV(r.Context(), groupID, userID, file, memberMapping)
	if err != nil {
		handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func isCSVFilename(filename string) bool {
	return len(filename) > 4 && filename[len(filename)-4:] == ".csv"
}
