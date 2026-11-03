package store

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "openpak/ps5-trophy/internal/achievementspb"
)

// Network is the account-core link namespace the achievements service files PlayStation
// trophies under (shared with np-trophy: a cross-gen set earned on PS4 shows on PS5).
const Network = "np"

const callTimeout = 5 * time.Second

// Achievements is the durable Store: trophy set definitions stay in the embedded Memory (loaded
// from the JSON files, read-only), every earned trophy lives in the neutral achievements core.
// The core keeps the first recorded unlock time, so a repeated Unlock never moves it.
type Achievements struct {
	*Memory
	Client pb.AchievementsClient
	Log    *slog.Logger
}

var _ Store = (*Achievements)(nil)

// DialAchievements connects to the achievements service (gRPC) and presents the internal key on
// every call. extra lets tests add a dialer.
func DialAchievements(addr, key string, log *slog.Logger, extra ...grpc.DialOption) (*Achievements, *grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, inv grpc.UnaryInvoker, o ...grpc.CallOption) error {
			return inv(metadata.AppendToOutgoingContext(ctx, "x-internal-key", key), method, req, reply, cc, o...)
		}),
	}, extra...)
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Achievements{Memory: NewMemory(), Client: pb.NewAchievementsClient(conn), Log: log}, conn, nil
}

func (a *Achievements) Unlock(account, id string, trophyID int, at time.Time) error {
	a.mu.RLock()
	s, ok := a.sets[id]
	a.mu.RUnlock()
	if !ok {
		return ErrNoSet
	}
	var trophy *Trophy
	group := ""
	for gi := range s.Groups {
		for ti := range s.Groups[gi].Trophies {
			if s.Groups[gi].Trophies[ti].ID == trophyID {
				trophy, group = &s.Groups[gi].Trophies[ti], s.Groups[gi].ID
			}
		}
	}
	if trophy == nil {
		return ErrNoTrophy
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_, err := a.Client.Unlock(ctx, &pb.UnlockRequest{
		Ref: &pb.TitleRef{Network: Network, Title: id}, Account: account,
		Unlocks: []*pb.UnlockItem{{AchievementId: strconv.Itoa(trophyID), UnlockedUnix: at.Unix(), Progress: 100, Grade: trophy.Type, GroupId: group}},
	})
	return err
}

func (a *Achievements) Earnings(account, id string) map[int]Earned {
	out := map[int]Earned{}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	res, err := a.Client.ListUnlocks(ctx, &pb.ListUnlocksRequest{Ref: &pb.TitleRef{Network: Network, Title: id}, Account: account})
	if err != nil {
		a.Log.Error("achievements read", "err", err)
		return out
	}
	titles := res.GetTitles()
	for _, t := range titles {
		// A title-scoped answer is one title; an unscoped one lists every title of the network.
		if len(titles) > 1 && t.GetNativeId() != id {
			continue
		}
		for _, u := range t.GetUnlocks() {
			n, err := strconv.Atoi(u.GetAchievementId())
			if err != nil || u.GetUnlockedUnix() == 0 {
				continue
			}
			out[n] = Earned{At: time.Unix(u.GetUnlockedUnix(), 0).UTC(), Progress: 100}
		}
	}
	return out
}

func (a *Achievements) Played(account string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	res, err := a.Client.ListUnlocks(ctx, &pb.ListUnlocksRequest{Ref: &pb.TitleRef{Network: Network}, Account: account})
	if err != nil {
		a.Log.Error("achievements read", "err", err)
		return nil
	}
	var out []string
	for _, t := range res.GetTitles() {
		if t.GetNativeId() == "" {
			continue
		}
		for _, u := range t.GetUnlocks() {
			if u.GetUnlockedUnix() != 0 {
				out = append(out, t.GetNativeId())
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
