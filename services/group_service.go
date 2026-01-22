package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"unwise-backend/database"
	apperrors "unwise-backend/errors"
	"unwise-backend/models"
	"unwise-backend/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type GroupService interface {
	GetByID(ctx context.Context, groupID, userID string) (*models.Group, error)
	GetByUserID(ctx context.Context, userID string) ([]models.Group, error)
	GetByUserIDWithBalances(ctx context.Context, userID string) ([]models.GroupWithBalances, error)
	Create(ctx context.Context, userID string, name string, groupType models.GroupType, memberEmails []string) (*models.Group, error)
	Update(ctx context.Context, groupID, userID string, name string) (*models.Group, error)
	UpdateGroupAvatar(ctx context.Context, groupID, userID, avatarURL string) (*models.Group, error)
	UpdateDefaultCurrency(ctx context.Context, groupID, userID, currency string) (*models.Group, error)
	Delete(ctx context.Context, groupID, userID string) error
	AddMember(ctx context.Context, groupID, userID, newMemberEmail string) error
	AddPlaceholderMember(ctx context.Context, groupID, userID, name string) error
	RemoveMember(ctx context.Context, groupID, userID, memberToRemoveID string) error
	GetTransactions(ctx context.Context, groupID, userID string) ([]models.Transaction, error)
	CreateRepayment(ctx context.Context, groupID, payerID, receiverID string, amount float64) (*models.Expense, error)
	CreateSettlement(ctx context.Context, groupID, requesterID, fromUserID, toUserID string, amount float64) (*models.Expense, error)
	GetBalances(ctx context.Context, groupID, userID string) (*models.GroupBalancesResponse, error)
	GetBalancesEdgeList(ctx context.Context, groupID, userID string) (*models.GroupBalancesEdgeResponse, error)
}

type groupService struct {
	groupRepo         repository.GroupRepository
	userRepo          repository.UserRepository
	expenseRepo       repository.ExpenseRepository
	settlementService SettlementService
	db                *database.DB
}

func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository, expenseRepo repository.ExpenseRepository, settlementService SettlementService, db *database.DB) GroupService {
	return &groupService{
		groupRepo:         groupRepo,
		userRepo:          userRepo,
		expenseRepo:       expenseRepo,
		settlementService: settlementService,
		db:                db,
	}
}

func (s *groupService) requireMembership(ctx context.Context, groupID, userID string) error {
	return RequireGroupMembership(ctx, s.groupRepo, groupID, userID)
}

func (s *groupService) GetByID(ctx context.Context, groupID, userID string) (*models.Group, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.GroupNotFound()
		}
		return nil, apperrors.DatabaseError("getting group", err)
	}

	balances, err := s.calculateBalances(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("calculating balances", err)
	}
	group.Balances = balances

	totalSpend, err := s.expenseRepo.GetGroupTotalSpend(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting total spend", err)
	}
	group.TotalSpend = math.Round(totalSpend*RoundingFactor) / RoundingFactor

	hasDebts := false
	for _, balance := range balances {
		if math.Abs(balance.OwedAmount) > BalanceThreshold {
			hasDebts = true
			break
		}
	}
	group.HasDebts = hasDebts

	for i := range group.Members {
		for _, balance := range balances {
			if balance.UserID == group.Members[i].ID {
				group.Members[i].Balance = balance.OwedAmount
				break
			}
		}
	}

	return group, nil
}

func (s *groupService) GetByUserID(ctx context.Context, userID string) ([]models.Group, error) {
	groups, err := s.groupRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting groups", err)
	}

	if groups == nil {
		groups = []models.Group{}
	}
	return groups, nil
}

func (s *groupService) GetByUserIDWithBalances(ctx context.Context, userID string) ([]models.GroupWithBalances, error) {
	groups, err := s.groupRepo.GetGroupsDetailedByUserID(ctx, userID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting detailed groups", err)
	}

	result := make([]models.GroupWithBalances, 0, len(groups))
	for _, group := range groups {
		var currentUserIDBalance float64
		membersWithBalance := make([]models.GroupMemberWithBalance, 0, len(group.Members))

		for _, member := range group.Members {
			roundedBalance := math.Round(member.Balance*RoundingFactor) / RoundingFactor
			if member.ID == userID {
				currentUserIDBalance = roundedBalance
			}
			membersWithBalance = append(membersWithBalance, models.GroupMemberWithBalance{
				ID:        member.ID,
				Name:      member.Name,
				Email:     member.Email,
				AvatarURL: member.AvatarURL,
				Balance:   roundedBalance,
			})
		}

		var state models.BalanceState
		if currentUserIDBalance > BalanceThreshold {
			state = models.BalanceStateOwed
		} else if currentUserIDBalance < -BalanceThreshold {
			state = models.BalanceStateOwes
		} else {
			state = models.BalanceStateSettled
		}

		result = append(result, models.GroupWithBalances{
			ID:           group.ID,
			Name:         group.Name,
			Type:         group.Type,
			CreatedAt:    group.CreatedAt,
			UpdatedAt:    group.UpdatedAt,
			Members:      membersWithBalance,
			MemberCount:  group.MemberCount,
			TotalBalance: math.Abs(currentUserIDBalance),
			Summary: models.GroupSummary{
				TotalNet: currentUserIDBalance,
				State:    state,
			},
		})
	}

	return result, nil
}

func (s *groupService) Create(ctx context.Context, userID string, name string, groupType models.GroupType, memberEmails []string) (*models.Group, error) {
	if groupType == "" {
		groupType = models.GroupTypeOther
	}

	group := &models.Group{
		ID:   uuid.New().String(),
		Name: name,
		Type: groupType,
	}

	err := s.db.WithTx(ctx, func(q database.Querier) error {
		txRepo := s.groupRepo.WithTx(q)
		if err := txRepo.Create(ctx, group); err != nil {
			return apperrors.DatabaseError("creating group", err)
		}

		if err := txRepo.AddMember(ctx, group.ID, userID); err != nil {
			return apperrors.DatabaseError("adding creator to group", err)
		}

		txUserRepo := s.userRepo.WithTx(q)
		for _, email := range memberEmails {
			user, err := txUserRepo.GetByEmail(ctx, email)
			if err != nil {
				if apperrors.IsNotFoundError(err) {
					return apperrors.UserNotFoundByEmail(email)
				}
				return apperrors.DatabaseError("finding user by email", err)
			}
			if user.ID != userID {
				if err := txRepo.AddMember(ctx, group.ID, user.ID); err != nil {
					return apperrors.DatabaseError("adding member to group", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.groupRepo.GetByID(ctx, group.ID)
}

func (s *groupService) Update(ctx context.Context, groupID, userID string, name string) (*models.Group, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.GroupNotFound()
		}
		return nil, apperrors.DatabaseError("getting group", err)
	}

	group.Name = name
	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, apperrors.DatabaseError("updating group", err)
	}

	return s.groupRepo.GetByID(ctx, groupID)
}

func (s *groupService) UpdateGroupAvatar(ctx context.Context, groupID, userID, avatarURL string) (*models.Group, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	if err := s.groupRepo.UpdateAvatarURL(ctx, groupID, avatarURL); err != nil {
		return nil, apperrors.DatabaseError("updating group avatar", err)
	}

	return s.groupRepo.GetByID(ctx, groupID)
}

func (s *groupService) UpdateDefaultCurrency(ctx context.Context, groupID, userID, currency string) (*models.Group, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	if len(currency) != 3 {
		return nil, apperrors.InvalidRequest("Currency code must be 3 characters")
	}

	if err := s.groupRepo.UpdateDefaultCurrency(ctx, groupID, currency); err != nil {
		return nil, apperrors.DatabaseError("updating group default currency", err)
	}

	return s.groupRepo.GetByID(ctx, groupID)
}

func (s *groupService) Delete(ctx context.Context, groupID, userID string) error {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return err
	}

	balances, err := s.calculateBalances(ctx, groupID)
	if err != nil {
		return apperrors.DatabaseError("calculating balances", err)
	}
	if len(balances) > 0 {
		return apperrors.CannotDeleteGroupWithDebts()
	}

	if err := s.groupRepo.Delete(ctx, groupID); err != nil {
		return apperrors.DatabaseError("deleting group", err)
	}

	return nil
}

func (s *groupService) AddMember(ctx context.Context, groupID, userID, newMemberEmail string) error {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return err
	}

	zap.L().Info("Adding member to group", zap.String("group_id", groupID), zap.String("requested_by", userID), zap.String("email", newMemberEmail))

	user, err := s.userRepo.GetByEmail(ctx, newMemberEmail)
	if err != nil {
		zap.L().Error("User lookup failed for email", zap.String("email", newMemberEmail), zap.Error(err))
		if apperrors.IsNotFoundError(err) {
			return apperrors.UserNotFoundByEmail(newMemberEmail)
		}
		return apperrors.DatabaseError("finding user by email", err)
	}

	zap.L().Info("Found user for group invitation", zap.String("email", user.Email), zap.String("user_id", user.ID), zap.String("group_id", groupID))

	if err := s.groupRepo.AddMember(ctx, groupID, user.ID); err != nil {
		zap.L().Error("Failed to add member to group", zap.String("user_id", user.ID), zap.String("group_id", groupID), zap.Error(err))
		if apperrors.IsDuplicateError(err) {
			return apperrors.AlreadyMember()
		}
		return apperrors.DatabaseError("adding member", err)
	}

	zap.L().Info("Successfully added member to group", zap.String("user_id", user.ID), zap.String("group_id", groupID))
	return nil
}

func (s *groupService) AddPlaceholderMember(ctx context.Context, groupID, userID, name string) error {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return err
	}

	newUserID := uuid.New().String()
	user := &models.User{
		ID:            newUserID,
		Name:          name,
		Email:         "",
		IsPlaceholder: true,
	}

	return s.db.WithTx(ctx, func(q database.Querier) error {
		txUserRepo := s.userRepo.WithTx(q)
		txGroupRepo := s.groupRepo.WithTx(q)

		if err := txUserRepo.Create(ctx, user); err != nil {
			return apperrors.DatabaseError("creating placeholder user", err)
		}

		if err := txGroupRepo.AddMember(ctx, groupID, newUserID); err != nil {
			return apperrors.DatabaseError("adding placeholder member", err)
		}
		return nil
	})
}

func (s *groupService) RemoveMember(ctx context.Context, groupID, userID, memberToRemoveID string) error {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return err
	}

	balances, err := s.calculateBalances(ctx, groupID)
	if err != nil {
		return apperrors.DatabaseError("calculating balances", err)
	}

	for _, b := range balances {
		if b.UserID == memberToRemoveID && math.Abs(b.OwedAmount) > BalanceThreshold {
			return apperrors.CannotRemoveMemberWithBalance(b.OwedAmount)
		}
	}

	if err := s.groupRepo.RemoveMember(ctx, groupID, memberToRemoveID); err != nil {
		return apperrors.DatabaseError("removing member", err)
	}

	return nil
}

func (s *groupService) GetTransactions(ctx context.Context, groupID, userID string) ([]models.Transaction, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	transactions, err := s.expenseRepo.GetTransactionsByGroupID(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting transactions", err)
	}

	enrichedTransactions := make([]models.Transaction, 0, len(transactions))
	userCache := make(map[string]*models.User)

	for _, t := range transactions {
		enriched := t

		if t.Category == models.TransactionCategoryPayment || t.Category == models.TransactionCategoryRepayment {
			enriched.Type = "repayment"
		} else {
			enriched.Type = "expense"
		}

		if t.PaidByUserID != nil {
			if t.PaidByUser == nil {
				paidByUser, err := s.getUserWithCache(ctx, *t.PaidByUserID, userCache)
				if err == nil {
					enriched.PaidByUser = paidByUser
				}
			}
		} else if len(t.Payers) > 0 {
			paidByUserID := t.Payers[0].UserID
			paidByUser, err := s.getUserWithCache(ctx, paidByUserID, userCache)
			if err == nil {
				enriched.PaidByUser = paidByUser
			}
		}

		var userSplitAmount float64
		var userPaidAmount float64
		var userIsPayer, userIsRecipient bool

		for _, payer := range t.Payers {
			if payer.UserID == userID {
				userIsPayer = true
				userPaidAmount = payer.AmountPaid
			}
		}

		if t.Category == models.TransactionCategoryPayment || t.Category == models.TransactionCategoryRepayment {
			for _, split := range t.Splits {
				if split.UserID == userID {
					userIsRecipient = true
					userSplitAmount = split.Amount
					break
				}
			}
		} else {
			for _, split := range t.Splits {
				if split.UserID == userID {
					userSplitAmount = split.Amount
					break
				}
			}
		}

		enriched.UserShare = math.Round(userSplitAmount*RoundingFactor) / RoundingFactor
		enriched.UserIsPayer = userIsPayer
		enriched.UserIsRecipient = userIsRecipient

		netAmount := userPaidAmount - userSplitAmount
		enriched.UserNetAmount = math.Round(netAmount*RoundingFactor) / RoundingFactor
		enriched.UserIsOwed = netAmount > BalanceThreshold
		enriched.UserIsLent = netAmount > BalanceThreshold

		for i := range enriched.Splits {
			splitUser, err := s.getUserWithCache(ctx, enriched.Splits[i].UserID, userCache)
			if err == nil {
				enriched.Splits[i].UserName = splitUser.Name
				enriched.Splits[i].UserEmail = splitUser.Email
			}
		}

		enrichedTransactions = append(enrichedTransactions, enriched)
	}

	return enrichedTransactions, nil
}

func (s *groupService) CreateRepayment(ctx context.Context, groupID, payerID, receiverID string, amount float64) (*models.Expense, error) {
	isMember, err := s.groupRepo.IsMember(ctx, groupID, payerID)
	if err != nil {
		return nil, apperrors.DatabaseError("checking membership", err)
	}
	if !isMember {
		return nil, apperrors.NotGroupMember()
	}

	isReceiverMember, err := s.groupRepo.IsMember(ctx, groupID, receiverID)
	if err != nil {
		return nil, apperrors.DatabaseError("checking receiver membership", err)
	}
	if !isReceiverMember {
		return nil, apperrors.NotGroupMember()
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group for currency", err)
	}
	currency := group.DefaultCurrency
	if currency == "" {
		currency = "INR"
	}

	expenseID := uuid.New().String()
	payerIDPtr := &payerID
	expense := &models.Expense{
		ID:           expenseID,
		GroupID:      groupID,
		PaidByUserID: payerIDPtr,
		TotalAmount:  amount,
		Currency:     currency,
		Description:  fmt.Sprintf("Repayment from %s to %s", payerID, receiverID),
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryRepayment,
		DateISO:      time.Now(),
		Date:         time.Now().Format("2006-01-02"),
		Time:         time.Now().Format("15:04"),
		Payers: []models.ExpensePayer{
			{
				ID:         uuid.New().String(),
				ExpenseID:  expenseID,
				UserID:     payerID,
				AmountPaid: amount,
			},
		},
	}

	err = s.db.WithTx(ctx, func(q database.Querier) error {
		txRepo := s.expenseRepo.WithTx(q)
		if err := txRepo.Create(ctx, expense); err != nil {
			return apperrors.DatabaseError("creating repayment", err)
		}

		for i := range expense.Payers {
			expense.Payers[i].ID = uuid.New().String()
			expense.Payers[i].ExpenseID = expenseID
			if err := txRepo.CreatePayer(ctx, &expense.Payers[i]); err != nil {
				return apperrors.DatabaseError("creating repayment payer", err)
			}
		}

		split := &models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenseID,
			UserID:    receiverID,
			Amount:    amount,
		}

		if err := txRepo.CreateSplit(ctx, split); err != nil {
			return apperrors.DatabaseError("creating repayment split", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.expenseRepo.GetByID(ctx, expenseID)
}

func (s *groupService) calculateBalances(ctx context.Context, groupID string) ([]models.Balance, error) {
	balancesByCurrency, err := s.expenseRepo.GetGroupMemberBalances(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group member balances", err)
	}

	var result []models.Balance
	for userID, currencyMap := range balancesByCurrency {
		for _, balance := range currencyMap {
			roundedBalance := math.Round(balance*RoundingFactor) / RoundingFactor
			if math.Abs(roundedBalance) > BalanceThreshold {
				result = append(result, models.Balance{
					UserID:     userID,
					OwedAmount: roundedBalance,
				})
			}
		}
	}

	return result, nil
}

func (s *groupService) GetBalances(ctx context.Context, groupID, userID string) (*models.GroupBalancesResponse, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	balancesByCurrency, err := s.expenseRepo.GetGroupMemberBalances(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group member balances", err)
	}

	settlements, err := s.settlementService.CalculateSettlements(ctx, groupID, userID)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("calculating settlements: %w", err))
	}

	owesToMap := make(map[string][]models.OwesToEntry)
	for _, s := range settlements {
		owesToMap[s.FromUserID] = append(owesToMap[s.FromUserID], models.OwesToEntry{
			UserID: s.ToUserID,
			Amount: s.Amount,
		})
	}

	userBalances := make([]models.UserBalance, 0)
	for uID, currencyMap := range balancesByCurrency {
		var totalBalance float64
		for _, balance := range currencyMap {
			totalBalance += balance
		}
		roundedBalance := math.Round(totalBalance*RoundingFactor) / RoundingFactor
		owesTo := owesToMap[uID]
		if len(owesTo) > 0 || math.Abs(roundedBalance) > BalanceThreshold {
			userBalances = append(userBalances, models.UserBalance{
				UserID:     uID,
				NetBalance: roundedBalance,
				OwesTo:     owesTo,
			})
		}
	}

	totalSpending, err := s.expenseRepo.GetGroupTotalSpend(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group total spend", err)
	}

	return &models.GroupBalancesResponse{
		TotalGroupSpending: math.Round(totalSpending*RoundingFactor) / RoundingFactor,
		UserBalances:       userBalances,
	}, nil
}

func (s *groupService) GetBalancesEdgeList(ctx context.Context, groupID, userID string) (*models.GroupBalancesEdgeResponse, error) {
	if err := s.requireMembership(ctx, groupID, userID); err != nil {
		return nil, err
	}

	balancesByCurrency, err := s.expenseRepo.GetGroupMemberBalances(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group member balances", err)
	}

	settlements, err := s.settlementService.CalculateSettlements(ctx, groupID, userID)
	if err != nil {
		return nil, apperrors.InternalError(fmt.Errorf("calculating settlements: %w", err))
	}

	var userNetBalance float64
	if userCurrencies, ok := balancesByCurrency[userID]; ok {
		for _, balance := range userCurrencies {
			userNetBalance += balance
		}
	}
	roundedBalance := math.Round(userNetBalance*RoundingFactor) / RoundingFactor

	var state models.BalanceState
	var totalOwedToUser, totalUserOwes float64
	var countOwedToUser, countUserOwes int

	if roundedBalance > BalanceThreshold {
		state = models.BalanceStateOwed
		totalOwedToUser = roundedBalance
	} else if roundedBalance < -BalanceThreshold {
		state = models.BalanceStateOwes
		totalUserOwes = math.Abs(roundedBalance)
	} else {
		state = models.BalanceStateSettled
	}

	debts := make([]models.DebtEdge, 0)
	userCache := make(map[string]*models.User)

	for _, settlement := range settlements {
		fromUser, err := s.getUserWithCache(ctx, settlement.FromUserID, userCache)
		if err != nil {
			return nil, apperrors.DatabaseError("getting from user", err)
		}

		toUser, err := s.getUserWithCache(ctx, settlement.ToUserID, userCache)
		if err != nil {
			return nil, apperrors.DatabaseError("getting to user", err)
		}

		debts = append(debts, models.DebtEdge{
			FromUser: models.UserInfo{
				ID:        fromUser.ID,
				Name:      fromUser.Name,
				AvatarURL: fromUser.AvatarURL,
			},
			ToUser: models.UserInfo{
				ID:        toUser.ID,
				Name:      toUser.Name,
				AvatarURL: toUser.AvatarURL,
			},
			Amount:   settlement.Amount,
			Currency: settlement.Currency,
		})

		if settlement.FromUserID == userID {
			countUserOwes++
		}
		if settlement.ToUserID == userID {
			countOwedToUser++
		}
	}

	return &models.GroupBalancesEdgeResponse{
		Summary: models.BalanceSummary{
			UserID:          userID,
			TotalNet:        roundedBalance,
			TotalOwedToUser: totalOwedToUser,
			TotalUserOwes:   totalUserOwes,
			CountOwedToUser: countOwedToUser,
			CountUserOwes:   countUserOwes,
			State:           state,
		},
		Debts: debts,
	}, nil
}

func (s *groupService) getUserWithCache(ctx context.Context, userID string, cache map[string]*models.User) (*models.User, error) {
	if user, ok := cache[userID]; ok {
		return user, nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.UserNotFound()
		}
		return nil, apperrors.DatabaseError("getting user", err)
	}

	cache[userID] = user
	return user, nil
}

func (s *groupService) CreateSettlement(ctx context.Context, groupID, requesterID, fromUserID, toUserID string, amount float64) (*models.Expense, error) {
	if amount <= 0 {
		return nil, apperrors.InvalidAmount("Amount must be greater than zero.")
	}

	isRequesterMember, err := s.groupRepo.IsMember(ctx, groupID, requesterID)
	if err != nil {
		return nil, apperrors.DatabaseError("checking requester membership", err)
	}
	if !isRequesterMember {
		return nil, apperrors.NotGroupMember()
	}

	isMember, err := s.groupRepo.IsMember(ctx, groupID, fromUserID)
	if err != nil {
		return nil, apperrors.DatabaseError("checking from user membership", err)
	}
	if !isMember {
		return nil, apperrors.Wrap(fmt.Errorf("from user is not a member"), apperrors.NotGroupMember())
	}

	isToMember, err := s.groupRepo.IsMember(ctx, groupID, toUserID)
	if err != nil {
		return nil, apperrors.DatabaseError("checking to user membership", err)
	}
	if !isToMember {
		return nil, apperrors.Wrap(fmt.Errorf("to user is not a member"), apperrors.NotGroupMember())
	}

	if fromUserID == toUserID {
		return nil, apperrors.CannotSettleToSelf()
	}

	fromUser, err := s.userRepo.GetByID(ctx, fromUserID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.UserNotFound()
		}
		return nil, apperrors.DatabaseError("getting from user", err)
	}

	toUser, err := s.userRepo.GetByID(ctx, toUserID)
	if err != nil {
		if apperrors.IsNotFoundError(err) {
			return nil, apperrors.UserNotFound()
		}
		return nil, apperrors.DatabaseError("getting to user", err)
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, apperrors.DatabaseError("getting group for currency", err)
	}
	currency := group.DefaultCurrency
	if currency == "" {
		currency = "INR"
	}

	expenseID := uuid.New().String()
	fromUserIDPtr := &fromUserID
	description := fmt.Sprintf("Payment from %s to %s", fromUser.Name, toUser.Name)

	expense := &models.Expense{
		ID:           expenseID,
		GroupID:      groupID,
		PaidByUserID: fromUserIDPtr,
		TotalAmount:  amount,
		Currency:     currency,
		Description:  description,
		Type:         models.ExpenseTypeEqual,
		Category:     models.TransactionCategoryPayment,
		DateISO:      time.Now(),
		Date:         time.Now().Format("2006-01-02"),
		Time:         time.Now().Format("15:04"),
		Payers: []models.ExpensePayer{
			{
				ID:         uuid.New().String(),
				ExpenseID:  expenseID,
				UserID:     fromUserID,
				AmountPaid: amount,
			},
		},
	}

	err = s.db.WithTx(ctx, func(q database.Querier) error {
		txRepo := s.expenseRepo.WithTx(q)
		if err := txRepo.Create(ctx, expense); err != nil {
			return apperrors.DatabaseError("creating payment transaction", err)
		}

		for i := range expense.Payers {
			if err := txRepo.CreatePayer(ctx, &expense.Payers[i]); err != nil {
				return apperrors.DatabaseError("creating payment payer", err)
			}
		}

		split := &models.ExpenseSplit{
			ID:        uuid.New().String(),
			ExpenseID: expenseID,
			UserID:    toUserID,
			Amount:    amount,
		}

		if err := txRepo.CreateSplit(ctx, split); err != nil {
			return apperrors.DatabaseError("creating payment split", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.expenseRepo.GetByID(ctx, expenseID)
}
