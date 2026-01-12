package handlers

import (
	"io"
	"log"
	"net/http"
	"time"

	apperrors "unwise-backend/errors"

	"github.com/google/uuid"
)

func (h *Handlers) ScanReceipt(w http.ResponseWriter, r *http.Request) {
	_, err := getUserID(r)
	if err != nil {
		log.Printf("[ScanReceipt] Failed to get user ID: %v", err)
		handleError(w, err)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		log.Printf("[ScanReceipt] Failed to parse multipart form: %v", err)
		handleError(w, apperrors.InvalidRequest("Failed to parse multipart form. Please ensure the request is properly formatted."))
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		log.Printf("[ScanReceipt] Failed to get image file: %v", err)
		handleError(w, apperrors.MissingRequiredField("Image file"))
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	filename := uuid.New().String() + "_" + time.Now().Format("20060102_150405")
	imageURL, err := h.storageService.Upload(r.Context(), h.storageBucket, filename, file, contentType)
	if err != nil {
		log.Printf("[ScanReceipt] Failed to upload image: %v", err)
		handleError(w, apperrors.StorageError("uploading receipt image", err))
		return
	}

	file.Seek(0, io.SeekStart)
	result, err := h.receiptService.ParseReceipt(r.Context(), file)
	if err != nil {
		log.Printf("[ScanReceipt] Gemini parsing failed: %v", err)
		handleError(w, apperrors.AIServiceError(err))
		return
	}

	response := map[string]interface{}{
		"receipt_image_url": imageURL,
		"items":             result.Items,
		"subtotal":          result.Subtotal,
		"tax":               result.Tax,
		"cgst":              result.CGST,
		"sgst":              result.SGST,
		"service_charge":    result.ServiceCharge,
		"total":             result.Total,
	}

	respondJSON(w, http.StatusOK, response)
}
