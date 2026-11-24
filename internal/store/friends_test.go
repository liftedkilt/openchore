package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

func TestSaveFriendSyncPendingConfirmAndPeerName(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	l := &model.FriendLink{
		Status:  model.FriendStatusWaiting,
		Side:    "a",
		HubURL:  "https://hub.example",
		HubLink: "L1",
		Token:   "tok1",
		Secret:  "sec1",
		Invite:  "ocf1.code",
	}
	if err := s.CreateFriendLink(ctx, l); err != nil {
		t.Fatalf("CreateFriendLink: %v", err)
	}

	// Waiting -> pending_confirm clears invite without requiring a snapshot.
	if err := s.SaveFriendSync(ctx, l.ID, model.FriendStatusPendingConfirm, "", "", nil, "", now); err != nil {
		t.Fatalf("SaveFriendSync pending_confirm: %v", err)
	}
	got, err := s.GetFriendLink(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.FriendStatusPendingConfirm || got.Invite != "" || got.PeerName != "" || got.Snapshot != "" || got.SnapshotAt != nil {
		t.Fatalf("after pending_confirm sync: %+v", got)
	}

	// Peer name alone updates peer_name while keeping snapshot empty.
	if err := s.SaveFriendSync(ctx, l.ID, model.FriendStatusPendingConfirm, "Garcias", "", nil, "", now); err != nil {
		t.Fatalf("SaveFriendSync peerName: %v", err)
	}
	got, _ = s.GetFriendLink(ctx, l.ID)
	if got.Status != model.FriendStatusPendingConfirm || got.PeerName != "Garcias" || got.Snapshot != "" || got.SnapshotAt != nil {
		t.Fatalf("after peer_name sync: %+v", got)
	}

	// SaveFriendSync must NEVER promote waiting or pending_confirm to active.
	if err := s.SaveFriendSync(ctx, l.ID, model.FriendStatusActive, "Garcias", "", nil, "", now); err != nil {
		t.Fatalf("SaveFriendSync active attempt: %v", err)
	}
	got, _ = s.GetFriendLink(ctx, l.ID)
	if got.Status != model.FriendStatusPendingConfirm {
		t.Fatalf("SaveFriendSync promoted pending_confirm to %q", got.Status)
	}

	// Error on waiting -> pending_confirm still clears invite and transitions status.
	l2 := &model.FriendLink{
		Status:  model.FriendStatusWaiting,
		Side:    "a",
		HubURL:  "https://hub.example",
		HubLink: "L2",
		Token:   "tok2",
		Secret:  "sec2",
		Invite:  "ocf1.code2",
	}
	if err := s.CreateFriendLink(ctx, l2); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveFriendSync(ctx, l2.ID, model.FriendStatusPendingConfirm, "", "", nil, "bad data", now); err != nil {
		t.Fatal(err)
	}
	got2, _ := s.GetFriendLink(ctx, l2.ID)
	if got2.Status != model.FriendStatusPendingConfirm || got2.Invite != "" || got2.LastError != "bad data" {
		t.Fatalf("error on pending_confirm: %+v", got2)
	}

	// Active link must NEVER regress to pending_confirm on a stale sync.
	ok, err := s.ConfirmFriendLink(ctx, l.ID)
	if err != nil || !ok {
		t.Fatalf("ConfirmFriendLink: ok=%v err=%v", ok, err)
	}
	if err := s.SaveFriendSync(ctx, l.ID, model.FriendStatusPendingConfirm, "Garcias", "", nil, "", now); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetFriendLink(ctx, l.ID)
	if got.Status != model.FriendStatusActive {
		t.Fatalf("active link regressed to %q", got.Status)
	}

	// Ended wipes snapshot and invite on pending_confirm link.
	if err := s.SaveFriendSync(ctx, l2.ID, model.FriendStatusEnded, "", "", nil, "", now); err != nil {
		t.Fatal(err)
	}
	got2, _ = s.GetFriendLink(ctx, l2.ID)
	if got2.Status != model.FriendStatusEnded || got2.Snapshot != "" || got2.Invite != "" {
		t.Fatalf("ended pending_confirm: %+v", got2)
	}
}

func TestConfirmFriendLink(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	makeLink := func(status, linkID, inv string) int64 {
		l := &model.FriendLink{
			Status:  status,
			Side:    "a",
			HubURL:  "https://hub.example",
			HubLink: linkID,
			Token:   "tok-" + linkID,
			Secret:  "sec-" + linkID,
			Invite:  inv,
		}
		if err := s.CreateFriendLink(ctx, l); err != nil {
			t.Fatalf("CreateFriendLink(%s): %v", status, err)
		}
		return l.ID
	}

	waitingID := makeLink(model.FriendStatusWaiting, "W", "ocf1.w")
	pendingID := makeLink(model.FriendStatusPendingConfirm, "P", "ocf1.p")
	endedID := makeLink(model.FriendStatusEnded, "E", "")

	// Only pending_confirm succeeds.
	if ok, err := s.ConfirmFriendLink(ctx, waitingID); err != nil || ok {
		t.Fatalf("confirm waiting = %v, %v; want false, nil", ok, err)
	}
	if ok, err := s.ConfirmFriendLink(ctx, endedID); err != nil || ok {
		t.Fatalf("confirm ended = %v, %v; want false, nil", ok, err)
	}
	if ok, err := s.ConfirmFriendLink(ctx, 99999); err != nil || ok {
		t.Fatalf("confirm missing = %v, %v; want false, nil", ok, err)
	}
	if ok, err := s.ConfirmFriendLink(ctx, pendingID); err != nil || !ok {
		t.Fatalf("confirm pending = %v, %v; want true, nil", ok, err)
	}
	got, _ := s.GetFriendLink(ctx, pendingID)
	if got.Status != model.FriendStatusActive || got.Invite != "" {
		t.Fatalf("after confirm: %+v", got)
	}
	// Second confirm on now-active link returns false.
	if ok, err := s.ConfirmFriendLink(ctx, pendingID); err != nil || ok {
		t.Fatalf("double confirm = %v, %v; want false, nil", ok, err)
	}
}

func TestListActiveFriendLinksFiltersNonActive(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	for i, status := range []string{
		model.FriendStatusWaiting,
		model.FriendStatusPendingConfirm,
		model.FriendStatusActive,
		model.FriendStatusEnded,
	} {
		l := &model.FriendLink{
			Status:  status,
			Side:    "a",
			HubURL:  "https://hub.example",
			HubLink: status,
			Token:   "tok",
			Secret:  "sec",
		}
		if err := s.CreateFriendLink(ctx, l); err != nil {
			t.Fatalf("link %d (%s): %v", i, status, err)
		}
	}
	all, err := s.ListFriendLinks(ctx)
	if err != nil || len(all) != 4 {
		t.Fatalf("ListFriendLinks = %d, %v; want 4", len(all), err)
	}
	active, err := s.ListActiveFriendLinks(ctx)
	if err != nil || len(active) != 1 || active[0].Status != model.FriendStatusActive {
		t.Fatalf("ListActiveFriendLinks = %+v, %v; want 1 active", active, err)
	}
}
