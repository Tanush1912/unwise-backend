package models

import (
	"time"
)

type User struct {
	ID            string     `json:"id" db:"id"`
	Email         string     `json:"email" db:"email"`
	Name          string     `json:"name" db:"name"`
	AvatarURL     *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	IsPlaceholder bool       `json:"is_placeholder" db:"is_placeholder"`
	ClaimedBy     *string    `json:"claimed_by,omitempty" db:"claimed_by"`
	ClaimedAt     *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	Balance       float64    `json:"balance,omitempty"`
}

type GroupType string

const (
	GroupTypeTrip   GroupType = "TRIP"
	GroupTypeHome   GroupType = "HOME"
	GroupTypeCouple GroupType = "COUPLE"
	GroupTypeOther  GroupType = "OTHER"
)

type Group struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Type        GroupType `json:"type" db:"type"`
	AvatarURL   *string   `json:"avatar_url,omitempty" db:"avatar_url"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	MemberCount int       `json:"member_count,omitempty" db:"member_count"`
	Members     []User    `json:"members,omitempty"`
	Balances    []Balance `json:"balances,omitempty"`
	TotalSpend  float64   `json:"total_spend,omitempty"`
	HasDebts    bool      `json:"has_debts,omitempty"`
}

type TransactionCategory string

const (
	TransactionCategoryExpense   TransactionCategory = "EXPENSE"
	TransactionCategoryRepayment TransactionCategory = "REPAYMENT"
	TransactionCategoryPayment   TransactionCategory = "PAYMENT"
)

type ExpenseType string

const (
	ExpenseTypeEqual       ExpenseType = "EQUAL"
	ExpenseTypePercentage  ExpenseType = "PERCENTAGE"
	ExpenseTypeItemized    ExpenseType = "ITEMIZED"
	ExpenseTypeExactAmount ExpenseType = "EXACT_AMOUNT"
)

type Expense struct {
	ID              string              `json:"id" db:"id"`
	GroupID         string              `json:"group_id" db:"group_id"`
	PaidByUserID    *string             `json:"paid_by_user_id,omitempty" db:"paid_by_user_id"`
	TotalAmount     float64             `json:"total_amount" db:"total_amount"`
	Description     string              `json:"description" db:"description"`
	ReceiptImageURL *string             `json:"receipt_image_url,omitempty" db:"receipt_image_url"`
	Type            ExpenseType         `json:"split_method" db:"type"`
	Category        TransactionCategory `json:"type" db:"category"`
	Tax             float64             `json:"tax" db:"tax"`
	CGST            float64             `json:"cgst" db:"cgst"`
	SGST            float64             `json:"sgst" db:"sgst"`
	ServiceCharge   float64             `json:"service_charge" db:"service_charge"`
	Explanation     *string             `json:"explanation,omitempty" db:"explanation"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
	DateISO         time.Time           `json:"date_iso" db:"transaction_timestamp"`
	Date            string              `json:"date" db:"date_only"`
	Time            string              `json:"time" db:"time_only"`
	Splits          []ExpenseSplit      `json:"splits,omitempty"`
	Payers          []ExpensePayer      `json:"payers,omitempty"`
	ReceiptItems    []ReceiptItem       `json:"receipt_items,omitempty"`
}

type ExpensePayer struct {
	ID         string    `json:"id" db:"id"`
	ExpenseID  string    `json:"expense_id" db:"expense_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	AmountPaid float64   `json:"amount_paid" db:"amount_paid"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type Transaction struct {
	Expense
	PaidByUser      *User   `json:"paid_by_user,omitempty"`
	Type            string  `json:"type,omitempty"`
	UserShare       float64 `json:"user_share,omitempty"`
	UserNetAmount   float64 `json:"user_net_amount,omitempty"`
	UserIsOwed      bool    `json:"user_is_owed,omitempty"`
	UserIsLent      bool    `json:"user_is_lent,omitempty"`
	UserIsPayer     bool    `json:"user_is_payer,omitempty"`
	UserIsRecipient bool    `json:"user_is_recipient,omitempty"`
}

type ExpenseSplit struct {
	ID         string    `json:"id" db:"id"`
	ExpenseID  string    `json:"expense_id" db:"expense_id"`
	UserID     string    `json:"user_id" db:"user_id"`
	Amount     float64   `json:"amount" db:"amount"`
	Percentage *float64  `json:"percentage,omitempty" db:"percentage"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	UserName   string    `json:"user_name,omitempty"`
	UserEmail  string    `json:"user_email,omitempty"`
}

type ReceiptItem struct {
	ID          string                  `json:"id" db:"id"`
	ExpenseID   string                  `json:"expense_id" db:"expense_id"`
	Name        string                  `json:"name" db:"name"`
	Price       float64                 `json:"price" db:"price"`
	CreatedAt   time.Time               `json:"created_at" db:"created_at"`
	Assignments []ReceiptItemAssignment `json:"assignments,omitempty"`
}

type ReceiptItemAssignment struct {
	ID            string    `json:"id" db:"id"`
	ReceiptItemID string    `json:"receipt_item_id" db:"receipt_item_id"`
	UserID        string    `json:"user_id" db:"user_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type Balance struct {
	UserID       string  `json:"user_id"`
	OwedAmount   float64 `json:"owed_amount"`
	OwedToUserID *string `json:"owed_to_user_id,omitempty"`
}

type GroupMemberWithBalance struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Balance   float64 `json:"balance"`
}

type GroupSummary struct {
	TotalNet float64      `json:"total_net"`
	State    BalanceState `json:"state"`
}

type GroupWithBalances struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Type         GroupType                `json:"type,omitempty"`
	CreatedAt    time.Time                `json:"created_at,omitempty"`
	UpdatedAt    time.Time                `json:"updated_at,omitempty"`
	Members      []GroupMemberWithBalance `json:"members"`
	Summary      GroupSummary             `json:"summary"`
	MemberCount  int                      `json:"member_count,omitempty"`
	TotalBalance float64                  `json:"total_balance,omitempty"`
}

type UserBalance struct {
	UserID     string        `json:"user_id"`
	NetBalance float64       `json:"net_balance"`
	OwesTo     []OwesToEntry `json:"owes_to"`
}

type OwesToEntry struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
}

type GroupBalancesResponse struct {
	TotalGroupSpending float64       `json:"total_group_spending"`
	UserBalances       []UserBalance `json:"user_balances"`
}

type BalanceState string

const (
	BalanceStateOwed    BalanceState = "OWED"
	BalanceStateOwes    BalanceState = "OWES"
	BalanceStateSettled BalanceState = "SETTLED"
)

type BalanceSummary struct {
	UserID          string       `json:"user_id"`
	TotalNet        float64      `json:"total_net"`
	TotalOwedToUser float64      `json:"total_owed_to_user"`
	TotalUserOwes   float64      `json:"total_user_owes"`
	CountOwedToUser int          `json:"count_owed_to_user"`
	CountUserOwes   int          `json:"count_user_owes"`
	State           BalanceState `json:"state"`
}

type DebtEdge struct {
	FromUser UserInfo `json:"from_user"`
	ToUser   UserInfo `json:"to_user"`
	Amount   float64  `json:"amount"`
}

type UserInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type GroupBalancesEdgeResponse struct {
	Summary BalanceSummary `json:"summary"`
	Debts   []DebtEdge     `json:"debts"`
}

type Settlement struct {
	FromUserID string  `json:"from_user_id"`
	ToUserID   string  `json:"to_user_id"`
	Amount     float64 `json:"amount"`
}

type ReceiptParseResult struct {
	Items            []ReceiptItemData `json:"items"`
	Subtotal         float64           `json:"subtotal"`
	Tax              float64           `json:"tax"`
	CGST             float64           `json:"cgst"`
	SGST             float64           `json:"sgst"`
	ServiceCharge    float64           `json:"service_charge"`
	Total            float64           `json:"total"`
	PricesIncludeTax bool              `json:"prices_include_tax"`
}

type ReceiptItemData struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type DashboardResponse struct {
	User           DashboardUserInfo   `json:"user"`
	Metrics        DashboardMetrics    `json:"metrics"`
	Groups         []DashboardGroup    `json:"groups"`
	RecentActivity []DashboardActivity `json:"recent_activity"`
}

type DashboardUserInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type DashboardMetrics struct {
	TotalNetBalance float64 `json:"total_net_balance"`
	TotalYouOwe     float64 `json:"total_you_owe"`
	TotalYouAreOwed float64 `json:"total_you_are_owed"`
}

type DashboardGroup struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	AvatarURL        *string   `json:"avatar_url,omitempty"`
	MyBalanceInGroup float64   `json:"my_balance_in_group"`
	LastActivityAt   time.Time `json:"last_activity_at"`
}

type Comment struct {
	ID        string            `json:"id" db:"id"`
	ExpenseID string            `json:"expense_id" db:"expense_id"`
	UserID    string            `json:"user_id" db:"user_id"`
	User      *User             `json:"user,omitempty"`
	Text      string            `json:"text" db:"text"`
	CreatedAt time.Time         `json:"created_at" db:"created_at"`
	Reactions []CommentReaction `json:"reactions,omitempty"`
}

type CommentReaction struct {
	ID        string    `json:"id" db:"id"`
	CommentID string    `json:"comment_id" db:"comment_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	User      *User     `json:"user,omitempty"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type DashboardActivity struct {
	ID              string    `json:"id"`
	Description     string    `json:"description"`
	Amount          float64   `json:"amount"`
	Type            string    `json:"type"`
	ActionText      string    `json:"action_text"`
	ReceiptImageURL *string   `json:"receipt_image_url,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	Date            time.Time `json:"date"`
}
type Friend struct {
	UserID    string    `json:"user_id" db:"user_id"`
	FriendID  string    `json:"friend_id" db:"friend_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type FriendGroupBalance struct {
	GroupID   string  `json:"group_id"`
	GroupName string  `json:"group_name"`
	Amount    float64 `json:"amount"`
}

type FriendWithBalance struct {
	UserInfo
	Email         string               `json:"email"`
	NetBalance    float64              `json:"net_balance"`
	Groups        []DashboardGroup     `json:"groups"`
	GroupBalances []FriendGroupBalance `json:"group_balances"`
}

type DebtExplanation struct {
	TransactionID string `json:"transaction_id"`
	Explanation   string `json:"explanation"`
}

type ExplanationRequest struct {
	TransactionID string `json:"transaction_id"`
}
