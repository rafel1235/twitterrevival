package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"radura.local/api/internal/db"
)

type Repository struct{ Pool *pgxpool.Pool }

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func profileFrom(row db.GetProfileByHandleRow) Profile {
	return Profile{ID: row.ID, Handle: row.Handle, DisplayName: row.DisplayName, Bio: row.Bio, AvatarStyle: row.AvatarStyle, LocationText: row.LocationText, WebsiteURL: row.WebsiteUrl, CreatedAt: row.CreatedAt.Time.UTC(), IsSelf: row.IsSelf, Following: row.Following, FollowedBy: row.FollowedBy, FollowersCount: row.FollowersCount, FollowingCount: row.FollowingCount}
}
func (r *Repository) Profile(ctx context.Context, viewer, handle string) (Profile, error) {
	row, err := db.New(r.Pool).GetProfileByHandle(ctx, db.GetProfileByHandleParams{ViewerID: viewer, Handle: handle})
	return profileFrom(row), mapError(err)
}
func (r *Repository) Update(ctx context.Context, userID, handle string, input UpdateInput) (Profile, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if _, err = q.LockActiveAccount(ctx, userID); err != nil {
		return Profile{}, mapError(err)
	}
	row, err := q.GetProfileByHandle(ctx, db.GetProfileByHandleParams{ViewerID: userID, Handle: handle})
	if err != nil {
		return Profile{}, mapError(err)
	}
	if input.DisplayName != nil {
		row.DisplayName = *input.DisplayName
	}
	if input.Bio != nil {
		row.Bio = *input.Bio
	}
	if input.AvatarStyle != nil {
		row.AvatarStyle = *input.AvatarStyle
	}
	if input.LocationText != nil {
		row.LocationText = *input.LocationText
	}
	if input.WebsiteURL != nil {
		row.WebsiteUrl = *input.WebsiteURL
	}
	err = q.UpdateOwnProfile(ctx, db.UpdateOwnProfileParams{UserID: userID, DisplayName: row.DisplayName, Bio: row.Bio, AvatarStyle: row.AvatarStyle, LocationText: row.LocationText, WebsiteUrl: row.WebsiteUrl})
	if err != nil {
		return Profile{}, err
	}
	if err = q.TouchUser(ctx, userID); err != nil {
		return Profile{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Profile{}, err
	}
	return profileFrom(row), nil
}
func (r *Repository) Mutate(ctx context.Context, actor, target, action string) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	if err = q.LockUserPair(ctx, db.LockUserPairParams{ActorID: actor, TargetID: target}); err != nil {
		return err
	}
	active, err := q.ActiveUserPair(ctx, db.ActiveUserPairParams{ActorID: actor, TargetID: target})
	if err != nil {
		return err
	}
	if !active {
		return ErrNotFound
	}
	switch action {
	case "follow":
		blocked, err := q.PairBlocked(ctx, db.PairBlockedParams{ActorID: actor, TargetID: target})
		if err != nil {
			return err
		}
		if blocked {
			return ErrBlocked
		}
		err = q.InsertFollow(ctx, db.InsertFollowParams{FollowerID: actor, FollowedID: target})
		if err != nil {
			return err
		}
	case "unfollow":
		err = q.DeleteFollow(ctx, db.DeleteFollowParams{FollowerID: actor, FollowedID: target})
	case "block":
		if err = q.InsertBlock(ctx, db.InsertBlockParams{BlockerID: actor, BlockedID: target}); err != nil {
			return err
		}
		err = q.RemovePairFollows(ctx, db.RemovePairFollowsParams{ActorID: actor, TargetID: target})
	case "unblock":
		err = q.DeleteBlock(ctx, db.DeleteBlockParams{BlockerID: actor, BlockedID: target})
	default:
		return errors.New("invalid relationship action")
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) Connections(ctx context.Context, viewer, target, kind string, p Pagination) ([]Connection, error) {
	q := db.New(r.Pool)
	if kind != "blocks" {
		if _, err := q.VisibleUserID(ctx, db.VisibleUserIDParams{TargetID: target, ViewerID: viewer}); err != nil {
			return nil, mapError(err)
		}
	}
	at := pgtype.Timestamptz{Time: p.Before, Valid: true}
	limit := p.Limit + 1
	result := make([]Connection, 0)
	switch kind {
	case "followers":
		rows, err := q.ListFollowers(ctx, db.ListFollowersParams{ViewerID: viewer, TargetID: target, CursorTime: at, CursorID: p.ID, PageLimit: limit})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			result = append(result, Connection{row.ID, row.Handle, row.DisplayName, row.AvatarStyle, row.ConnectionCreatedAt.Time.UTC()})
		}
	case "following":
		rows, err := q.ListFollowing(ctx, db.ListFollowingParams{ViewerID: viewer, TargetID: target, CursorTime: at, CursorID: p.ID, PageLimit: limit})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			result = append(result, Connection{row.ID, row.Handle, row.DisplayName, row.AvatarStyle, row.ConnectionCreatedAt.Time.UTC()})
		}
	case "blocks":
		rows, err := q.ListBlocked(ctx, db.ListBlockedParams{ViewerID: viewer, CursorTime: at, CursorID: p.ID, PageLimit: limit})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			result = append(result, Connection{row.ID, row.Handle, row.DisplayName, row.AvatarStyle, row.ConnectionCreatedAt.Time.UTC()})
		}
	default:
		return nil, errors.New("invalid connection list")
	}
	return result, nil
}
