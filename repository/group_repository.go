package repository

import (
	"context"
	"fmt"
	"time"

	"unwise-backend/database"
	"unwise-backend/models"
)

type GroupRepository interface {
	GetByID(ctx context.Context, id string) (*models.Group, error)
	GetByUserID(ctx context.Context, userID string) ([]models.Group, error)
	GetGroupsWithLastActivity(ctx context.Context, userID string) ([]models.DashboardGroup, error)
	Create(ctx context.Context, group *models.Group) error
	Update(ctx context.Context, group *models.Group) error
	UpdateAvatarURL(ctx context.Context, groupID string, avatarURL string) error
	UpdateDefaultCurrency(ctx context.Context, groupID string, currency string) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, groupID, userID string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	GetMembers(ctx context.Context, groupID string) ([]models.User, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetCommonGroups(ctx context.Context, userID1, userID2 string) ([]models.Group, error)
	GetGroupsDetailedByUserID(ctx context.Context, userID string) ([]models.Group, error)
	WithTx(tx database.Querier) GroupRepository
}

type groupRepository struct {
	db *database.DB
	tx database.Querier
}

func NewGroupRepository(db *database.DB) GroupRepository {
	return &groupRepository{db: db}
}

func (r *groupRepository) WithTx(tx database.Querier) GroupRepository {
	return &groupRepository{db: r.db, tx: tx}
}

func (r *groupRepository) getQuerier() database.Querier {
	if r.tx != nil {
		return r.tx
	}
	return r.db.Pool
}

func (r *groupRepository) GetByID(ctx context.Context, id string) (*models.Group, error) {
	var group models.Group
	query := `SELECT id, name, type, default_currency, avatar_url, created_at, updated_at FROM groups WHERE id = $1`

	err := r.getQuerier().QueryRow(ctx, query, id).Scan(
		&group.ID, &group.Name, &group.Type, &group.DefaultCurrency, &group.AvatarURL, &group.CreatedAt, &group.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting group by id: %w", err)
	}

	members, err := r.GetMembers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting group members: %w", err)
	}
	group.Members = members

	return &group, nil
}

func (r *groupRepository) GetByUserID(ctx context.Context, userID string) ([]models.Group, error) {
	query := `SELECT 
	          g.id, 
	          g.name, 
	          g.type, 
	          g.default_currency,
	          g.avatar_url,
	          g.created_at, 
	          g.updated_at
	          FROM groups g
	          INNER JOIN group_members gm ON g.id = gm.group_id
	          WHERE gm.user_id = $1
	          ORDER BY g.updated_at DESC`

	rows, err := r.getQuerier().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("getting groups by user id: %w", err)
	}
	defer rows.Close()

	var groups []models.Group
	groupIDs := make([]string, 0)
	groupMap := make(map[string]*models.Group)

	for rows.Next() {
		var group models.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Type, &group.DefaultCurrency, &group.AvatarURL, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning group: %w", err)
		}
		group.Members = []models.User{}
		groups = append(groups, group)
		groupIDs = append(groupIDs, group.ID)
	}

	if len(groupIDs) == 0 {
		return []models.Group{}, nil
	}

	for i := range groups {
		groupMap[groups[i].ID] = &groups[i]
	}

	memberQuery := `SELECT u.id, COALESCE(u.email, ''), u.name, u.avatar_url, u.is_placeholder, u.claimed_by, u.claimed_at, u.created_at, u.updated_at, gm.group_id
	          FROM users u
	          INNER JOIN group_members gm ON u.id = gm.user_id
	          WHERE gm.group_id = ANY($1)`

	mRows, err := r.getQuerier().Query(ctx, memberQuery, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("getting batch members: %w", err)
	}
	defer mRows.Close()

	for mRows.Next() {
		var user models.User
		var groupID string
		var email string
		if err := mRows.Scan(
			&user.ID, &email, &user.Name, &user.AvatarURL, &user.IsPlaceholder,
			&user.ClaimedBy, &user.ClaimedAt, &user.CreatedAt, &user.UpdatedAt, &groupID,
		); err != nil {
			return nil, fmt.Errorf("scanning member: %w", err)
		}
		user.Email = email
		if g, ok := groupMap[groupID]; ok {
			g.Members = append(g.Members, user)
			g.MemberCount++
		}
	}

	return groups, nil
}

func (r *groupRepository) Create(ctx context.Context, group *models.Group) error {
	groupType := group.Type
	if groupType == "" {
		groupType = models.GroupTypeOther
	}

	query := `INSERT INTO groups (id, name, type, created_at, updated_at)
	          VALUES ($1, $2, $3, NOW(), NOW())`

	_, err := r.getQuerier().Exec(ctx, query, group.ID, group.Name, groupType)
	if err != nil {
		return fmt.Errorf("creating group: %w", err)
	}
	return nil
}

func (r *groupRepository) Update(ctx context.Context, group *models.Group) error {
	query := `UPDATE groups SET name = $1, updated_at = NOW() WHERE id = $2`

	_, err := r.getQuerier().Exec(ctx, query, group.Name, group.ID)
	if err != nil {
		return fmt.Errorf("updating group: %w", err)
	}
	return nil
}

func (r *groupRepository) UpdateAvatarURL(ctx context.Context, groupID string, avatarURL string) error {
	query := `UPDATE groups SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.getQuerier().Exec(ctx, query, avatarURL, groupID)
	if err != nil {
		return fmt.Errorf("updating group avatar: %w", err)
	}
	return nil
}

func (r *groupRepository) UpdateDefaultCurrency(ctx context.Context, groupID string, currency string) error {
	query := `UPDATE groups SET default_currency = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.getQuerier().Exec(ctx, query, currency, groupID)
	if err != nil {
		return fmt.Errorf("updating group default currency: %w", err)
	}
	return nil
}

func (r *groupRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM groups WHERE id = $1`

	_, err := r.getQuerier().Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting group: %w", err)
	}
	return nil
}

func (r *groupRepository) AddMember(ctx context.Context, groupID, userID string) error {
	query := `INSERT INTO group_members (group_id, user_id, created_at)
	          VALUES ($1, $2, NOW())
	          ON CONFLICT (group_id, user_id) DO NOTHING`

	_, err := r.getQuerier().Exec(ctx, query, groupID, userID)
	if err != nil {
		return fmt.Errorf("adding member to group: %w", err)
	}
	return nil
}

func (r *groupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`

	_, err := r.getQuerier().Exec(ctx, query, groupID, userID)
	if err != nil {
		return fmt.Errorf("removing member from group: %w", err)
	}
	return nil
}

func (r *groupRepository) GetMembers(ctx context.Context, groupID string) ([]models.User, error) {
	query := `SELECT u.id, COALESCE(u.email, ''), u.name, u.avatar_url, u.is_placeholder, u.claimed_by, u.claimed_at, u.created_at, u.updated_at
	          FROM users u
	          INNER JOIN group_members gm ON u.id = gm.user_id
	          WHERE gm.group_id = $1`

	rows, err := r.getQuerier().Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("getting group members: %w", err)
	}
	defer rows.Close()

	var members []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.IsPlaceholder,
			&user.ClaimedBy, &user.ClaimedAt, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning member: %w", err)
		}
		members = append(members, user)
	}

	return members, nil
}

func (r *groupRepository) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`

	err := r.getQuerier().QueryRow(ctx, query, groupID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking membership: %w", err)
	}
	return exists, nil
}

func (r *groupRepository) GetGroupsWithLastActivity(ctx context.Context, userID string) ([]models.DashboardGroup, error) {
	query := `SELECT 
	          g.id, 
	          g.name,
	          g.avatar_url,
	          COALESCE(MAX(e.created_at), g.updated_at) as last_activity_at
	          FROM groups g
	          INNER JOIN group_members gm ON g.id = gm.group_id
	          LEFT JOIN expenses e ON g.id = e.group_id
	          WHERE gm.user_id = $1
	          GROUP BY g.id, g.name, g.avatar_url, g.updated_at
	          ORDER BY last_activity_at DESC`

	rows, err := r.getQuerier().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("getting groups with last activity: %w", err)
	}
	defer rows.Close()

	var groups []models.DashboardGroup
	for rows.Next() {
		var group models.DashboardGroup
		if err := rows.Scan(&group.ID, &group.Name, &group.AvatarURL, &group.LastActivityAt); err != nil {
			return nil, fmt.Errorf("scanning group: %w", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

func (r *groupRepository) GetGroupsDetailedByUserID(ctx context.Context, userID string) ([]models.Group, error) {
	query := `
		WITH user_groups AS (
			SELECT group_id FROM group_members WHERE user_id = $1
		),
		payments AS (
			SELECT e.group_id, p.user_id, SUM(p.amount_paid) as paid
			FROM expense_payers p
			JOIN expenses e ON p.expense_id = e.id
			WHERE e.group_id IN (SELECT group_id FROM user_groups)
			GROUP BY e.group_id, p.user_id
		),
		splits AS (
			SELECT e.group_id, s.user_id, SUM(s.amount) as owed
			FROM expense_splits s
			JOIN expenses e ON s.expense_id = e.id
			WHERE e.group_id IN (SELECT group_id FROM user_groups)
			GROUP BY e.group_id, s.user_id
		)
		SELECT 
			g.id as g_id, g.name as g_name, g.type as g_type, g.avatar_url as g_avatar_url, 
			g.created_at as g_created_at, g.updated_at as g_updated_at,
			u.id as u_id, COALESCE(u.email, '') as u_email, u.name as u_name, 
			u.avatar_url as u_avatar_url, u.is_placeholder as u_is_placeholder,
			u.claimed_by as u_claimed_by, u.claimed_at as u_claimed_at,
			u.created_at as u_created_at, u.updated_at as u_updated_at,
			COALESCE(p.paid, 0) - COALESCE(s.owed, 0) as u_balance
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		JOIN users u ON gm.user_id = u.id
		LEFT JOIN payments p ON g.id = p.group_id AND u.id = p.user_id
		LEFT JOIN splits s ON g.id = s.group_id AND u.id = s.user_id
		WHERE g.id IN (SELECT group_id FROM user_groups)
		ORDER BY g.updated_at DESC, u.name ASC
	`

	rows, err := r.getQuerier().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("getting groups detailed: %w", err)
	}
	defer rows.Close()

	groupMap := make(map[string]*models.Group)
	var groupOrder []string

	for rows.Next() {
		var gID, gName, gType, uID, uEmail, uName string
		var gAvatarURL, uAvatarURL, uClaimedBy *string
		var gCreatedAt, gUpdatedAt, uCreatedAt, uUpdatedAt time.Time
		var uClaimedAt *time.Time
		var uIsPlaceholder bool
		var uBalance float64

		if err := rows.Scan(
			&gID, &gName, &gType, &gAvatarURL, &gCreatedAt, &gUpdatedAt,
			&uID, &uEmail, &uName, &uAvatarURL, &uIsPlaceholder,
			&uClaimedBy, &uClaimedAt, &uCreatedAt, &uUpdatedAt,
			&uBalance,
		); err != nil {
			return nil, fmt.Errorf("scanning detailed group: %w", err)
		}

		group, exists := groupMap[gID]
		if !exists {
			group = &models.Group{
				ID:        gID,
				Name:      gName,
				Type:      models.GroupType(gType),
				AvatarURL: gAvatarURL,
				CreatedAt: gCreatedAt,
				UpdatedAt: gUpdatedAt,
				Members:   []models.User{},
			}
			groupMap[gID] = group
			groupOrder = append(groupOrder, gID)
		}

		group.Members = append(group.Members, models.User{
			ID:            uID,
			Email:         uEmail,
			Name:          uName,
			AvatarURL:     uAvatarURL,
			IsPlaceholder: uIsPlaceholder,
			ClaimedBy:     uClaimedBy,
			ClaimedAt:     uClaimedAt,
			CreatedAt:     uCreatedAt,
			UpdatedAt:     uUpdatedAt,
			Balance:       uBalance,
		})
	}

	result := make([]models.Group, 0, len(groupOrder))
	for _, id := range groupOrder {
		g := groupMap[id]
		g.MemberCount = len(g.Members)
		result = append(result, *g)
	}

	return result, nil
}

func (r *groupRepository) GetCommonGroups(ctx context.Context, userID1, userID2 string) ([]models.Group, error) {
	query := `
		SELECT g.id, g.name, g.avatar_url, g.created_at, g.updated_at, g.type
		FROM groups g
		JOIN group_members gm1 ON g.id = gm1.group_id
		JOIN group_members gm2 ON g.id = gm2.group_id
		WHERE gm1.user_id = $1 AND gm2.user_id = $2
	`
	rows, err := r.db.Pool.Query(ctx, query, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("getting common groups: %w", err)
	}
	defer rows.Close()

	var groups []models.Group
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.AvatarURL, &g.CreatedAt, &g.UpdatedAt, &g.Type); err != nil {
			return nil, fmt.Errorf("scanning group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, nil
}
