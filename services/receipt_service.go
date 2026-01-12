package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"unwise-backend/models"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type ReceiptService interface {
	ParseReceipt(ctx context.Context, imageData io.Reader) (*models.ReceiptParseResult, error)
}

type receiptService struct {
	client *genai.Client
}

func NewReceiptService(apiKey string) (ReceiptService, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("creating gemini client: %w", err)
	}

	return &receiptService{client: client}, nil
}

func (s *receiptService) ParseReceipt(ctx context.Context, imageData io.Reader) (*models.ReceiptParseResult, error) {
	model := s.client.GenerativeModel("gemini-2.0-flash")

	systemPrompt := `Extract all items and the financial summary from this receipt.

CRITICAL: Determine if the item prices shown INCLUDE tax or are PRE-TAX amounts:
- If (sum of item prices) ≈ Total: prices ALREADY INCLUDE tax → set "prices_include_tax": true
- If (sum of item prices) ≈ Subtotal (and Subtotal + Tax ≈ Total): prices are PRE-TAX → set "prices_include_tax": false
- If unsure, compare the sum of item prices to both subtotal and total to determine which is closer.

Pay special attention to Indian tax structures like CGST, SGST, GST, Service Charge, and CESS.

Return ONLY valid JSON in this format:
{
  "items": [{ "name": "string", "price": number }],
  "subtotal": number,
  "tax": number,
  "cgst": number,
  "sgst": number,
  "service_charge": number,
  "total": number,
  "prices_include_tax": boolean
}

Rules:
- "tax" should be the sum of all taxes (CGST + SGST + CESS, etc.) if listed separately.
- If any field is not on the receipt, set its value to 0.
- "prices_include_tax" is REQUIRED - analyze the receipt carefully to determine this.
- Do not include markdown formatting, code blocks, or additional text. Only return raw JSON.`

	prompt := genai.Text(systemPrompt)

	imageBytes, err := io.ReadAll(imageData)
	if err != nil {
		return nil, fmt.Errorf("reading image data: %w", err)
	}

	imagePart := genai.ImageData("image/jpeg", imageBytes)

	resp, err := model.GenerateContent(ctx, prompt, imagePart)
	if err != nil {
		log.Printf("[ReceiptService.ParseReceipt] Gemini API call failed: %v", err)
		return nil, fmt.Errorf("generating content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from gemini")
	}

	text := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if textPart, ok := part.(genai.Text); ok {
			text += string(textPart)
		}
	}

	text = cleanJSONResponse(text)

	var result models.ReceiptParseResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}
	return &result, nil
}

func cleanJSONResponse(text string) string {
	text = removeMarkdownCodeBlocks(text)
	text = removeWhitespace(text)
	return text
}

func removeMarkdownCodeBlocks(text string) string {
	start := 0
	end := len(text)

	if idx := findString(text, "```json"); idx != -1 {
		start = idx + 7
	} else if idx := findString(text, "```"); idx != -1 {
		start = idx + 3
	}

	if idx := findString(text[start:], "```"); idx != -1 {
		end = start + idx
	}

	text = text[start:end]

	for len(text) > 0 && text[0] == '`' {
		text = text[1:]
	}
	for len(text) > 0 && text[len(text)-1] == '`' {
		text = text[:len(text)-1]
	}

	return text
}

func removeWhitespace(text string) string {
	var result []byte
	inString := false
	escapeNext := false

	for i := 0; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			result = append(result, char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			result = append(result, char)
			continue
		}

		if char == '"' {
			inString = !inString
			result = append(result, char)
			continue
		}

		if inString {
			result = append(result, char)
		} else if char != ' ' && char != '\n' && char != '\t' && char != '\r' {
			result = append(result, char)
		}
	}

	return string(result)
}

func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
