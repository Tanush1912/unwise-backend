package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	apperrors "unwise-backend/errors"
	"unwise-backend/middleware"
	"unwise-backend/services"
	"unwise-backend/storage"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ErrorResponse struct {
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

type Handlers struct {
	groupService       services.GroupService
	expenseService     services.ExpenseService
	settlementService  services.SettlementService
	receiptService     services.ReceiptService
	dashboardService   services.DashboardService
	userService        services.UserService
	explanationService services.ExplanationService
	friendService      services.FriendService
	commentService     services.CommentService
	storageService     storage.Storage
	storageBucket      string
	groupPhotosBucket  string
	userAvatarsBucket  string
}

func NewHandlers(
	groupService services.GroupService,
	expenseService services.ExpenseService,
	settlementService services.SettlementService,
	receiptService services.ReceiptService,
	dashboardService services.DashboardService,
	userService services.UserService,
	explanationService services.ExplanationService,
	friendService services.FriendService,
	commentService services.CommentService,
	storageService storage.Storage,
	storageBucket string,
	groupPhotosBucket string,
	userAvatarsBucket string,
) *Handlers {
	return &Handlers{
		groupService:       groupService,
		expenseService:     expenseService,
		settlementService:  settlementService,
		receiptService:     receiptService,
		dashboardService:   dashboardService,
		userService:        userService,
		explanationService: explanationService,
		friendService:      friendService,
		commentService:     commentService,
		storageService:     storageService,
		storageBucket:      storageBucket,
		groupPhotosBucket:  groupPhotosBucket,
		userAvatarsBucket:  userAvatarsBucket,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/dashboard", h.GetDashboard)

	r.Route("/friends", func(r chi.Router) {
		r.Get("/", h.GetFriends)
		r.Get("/search", h.SearchPotentialFriends)
		r.Post("/", h.AddFriend)
		r.Delete("/{friendID}", h.RemoveFriend)
	})

	r.Route("/groups", func(r chi.Router) {
		r.Get("/", h.GetGroups)
		r.Post("/", h.CreateGroup)
		r.Get("/{groupID}", h.GetGroup)
		r.Put("/{groupID}", h.UpdateGroup)
		r.Delete("/{groupID}", h.DeleteGroup)
		r.Post("/{groupID}/members", h.AddMember)
		r.Post("/{groupID}/placeholders", h.AddPlaceholderMember)
		r.Delete("/{groupID}/members/{userID}", h.RemoveMember)
		r.Get("/{groupID}/expenses", h.GetExpenses)
		r.Get("/{groupID}/transactions", h.GetTransactions)
		r.Get("/{groupID}/export", h.ExportGroupCSV)
		r.Get("/{groupID}/balances", h.GetBalances)
		r.Post("/{groupID}/settle", h.SettleUp)
		r.Get("/{groupID}/settlements", h.GetSettlements)
		r.Post("/{groupID}/avatar", h.UploadGroupAvatar)
	})

	r.Route("/expenses", func(r chi.Router) {
		r.Post("/", h.CreateExpense)
		r.Get("/{expenseID}", h.GetExpense)
		r.Put("/{expenseID}", h.UpdateExpense)
		r.Delete("/{expenseID}", h.DeleteExpense)
		r.Get("/{expenseID}/comments", h.GetComments)
		r.Post("/{expenseID}/comments", h.CreateComment)
		r.Delete("/{expenseID}/comments/{commentID}", h.DeleteComment)
		r.Post("/{expenseID}/comments/{commentID}/reactions", h.AddReaction)
		r.Delete("/{expenseID}/comments/{commentID}/reactions", h.RemoveReaction)
	})

	r.Route("/user", func(r chi.Router) {
		r.Get("/me", h.GetCurrentUser)
		r.Post("/avatar", h.UploadUserAvatar)
		r.Delete("/me", h.DeleteAccount)
		r.Get("/placeholders", h.GetClaimablePlaceholders)
		r.Post("/placeholders/{placeholderID}/claim", h.ClaimPlaceholder)
		r.Post("/placeholders/{placeholderID}/assign", h.AssignPlaceholder)
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		zap.L().Error("Failed to encode JSON response", zap.Error(err))
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	if status >= 500 {
		zap.L().Error("Server Error", zap.Int("status", status), zap.String("message", message))
	}
	respondJSON(w, status, ErrorResponse{Error: message})
}

func handleError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	if appErr, ok := apperrors.AsAppError(err); ok {
		status := apperrors.GetHTTPStatus(appErr.Type)

		if status >= 500 {
			zap.L().Error("App Error (Internal)",
				zap.String("code", string(appErr.Code)),
				zap.Error(appErr.Err))
		} else {
			zap.L().Debug("App Error (Client)",
				zap.String("code", string(appErr.Code)),
				zap.String("message", appErr.Message))
		}

		respondJSON(w, status, ErrorResponse{
			Error:   appErr.Message,
			Code:    string(appErr.Code),
			Details: appErr.Details,
		})
		return
	}

	zap.L().Error("Non-AppError returned (bug)",
		zap.Error(err),
		zap.String("error_type", fmt.Sprintf("%T", err)))

	respondJSON(w, http.StatusInternalServerError, ErrorResponse{
		Error: "An unexpected error occurred. Please try again later.",
		Code:  string(apperrors.CodeInternalError),
	})
}

func getUserID(r *http.Request) (string, error) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		return "", apperrors.Unauthorized("User ID not found in authentication context")
	}
	return userID, nil
}

func getUserEmail(r *http.Request) (string, error) {
	email, ok := middleware.GetUserEmail(r.Context())
	if !ok {
		return "", apperrors.Unauthorized("User email not found in authentication context")
	}
	return email, nil
}

func getUserName(r *http.Request) (string, error) {
	name, _ := middleware.GetUserName(r.Context())
	return name, nil
}
