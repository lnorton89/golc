// model_test.go proves Pool identity stability and count-independence
// (02-04-PLAN.md, Task 1 Wave-0 scaffold): a Pool's UUID survives a
// rename, two pools may share a name only if creation of the duplicate
// name is rejected, and a Pool with 0, 1, or ~50 members is equally
// valid.
//
// This file fails at build time until internal/pool exists (Task 2) --
// that is the RED state this task proves.
package pool_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/lnorton89/golc/internal/pool"
)

func TestPoolIdentityStable(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	originalID := p.ID

	renamed, err := pool.Rename(p, "Wash Pool Renamed")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.ID != originalID {
		t.Fatalf("expected ID to survive rename, got %s want %s", renamed.ID, originalID)
	}
	if renamed.Name != "Wash Pool Renamed" {
		t.Fatalf("expected renamed pool to carry its new name, got %q", renamed.Name)
	}

	// Two pools may share a name only if creation of the duplicate is
	// rejected (GOLC_POOL_DUPLICATE_NAME) -- never a silent duplicate.
	other, err := pool.NewPool(p.Name, nil)
	if err != nil {
		t.Fatalf("NewPool (second, same name): %v", err)
	}
	if err := pool.ValidateUniqueNames([]pool.Pool{p, other}); err == nil || !strings.Contains(err.Error(), "GOLC_POOL_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_POOL_DUPLICATE_NAME for duplicate pool names, got %v", err)
	}
}

func TestGroupUniqueNamesRejected(t *testing.T) {
	first := pool.Group{Name: "Front Wash"}
	second := pool.Group{Name: "Front Wash"}
	if err := pool.ValidateUniqueGroupNames([]pool.Group{first, second}); err == nil || !strings.Contains(err.Error(), "GOLC_GROUP_DUPLICATE_NAME") {
		t.Fatalf("expected GOLC_GROUP_DUPLICATE_NAME for duplicate group names, got %v", err)
	}
	if err := pool.ValidateUniqueGroupNames([]pool.Group{first, {Name: "Back Wash"}}); err != nil {
		t.Fatalf("expected distinctly named groups to be valid, got %v", err)
	}
}

func TestGroupReferencesValidated(t *testing.T) {
	p, err := pool.NewPool("Wash Pool", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	p.Members = append(p.Members, member)

	valid := pool.Group{
		Name:       "Front Wash",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: member.ID}},
	}
	if err := pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{valid}); err != nil {
		t.Fatalf("expected a group referencing a real pool member to be valid, got %v", err)
	}

	danglingPool := pool.Group{
		Name:       "Dangling Pool Ref",
		MemberRefs: []pool.MemberRef{{PoolID: uuid.Must(uuid.NewV7()), PoolMemberID: member.ID}},
	}
	if err := pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{danglingPool}); err == nil || !strings.Contains(err.Error(), "GOLC_GROUP_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_GROUP_DANGLING_REFERENCE for a reference to a nonexistent pool, got %v", err)
	}

	danglingMember := pool.Group{
		Name:       "Dangling Member Ref",
		MemberRefs: []pool.MemberRef{{PoolID: p.ID, PoolMemberID: uuid.Must(uuid.NewV7())}},
	}
	if err := pool.ValidateGroupReferences([]pool.Pool{p}, []pool.Group{danglingMember}); err == nil || !strings.Contains(err.Error(), "GOLC_GROUP_DANGLING_REFERENCE") {
		t.Fatalf("expected GOLC_GROUP_DANGLING_REFERENCE for a reference to a nonexistent pool member, got %v", err)
	}
}

func TestPoolCountIndependent(t *testing.T) {
	zero, err := pool.NewPool("Zero Members", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	if err := pool.Validate(zero); err != nil {
		t.Fatalf("expected a zero-member pool to be valid: %v", err)
	}

	one, err := pool.NewPool("One Member", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	member, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
	if err != nil {
		t.Fatalf("NewPoolMember: %v", err)
	}
	one.Members = append(one.Members, member)
	if err := pool.Validate(one); err != nil {
		t.Fatalf("expected a one-member pool to be valid: %v", err)
	}

	many, err := pool.NewPool("Fifty Members", nil)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	for i := 0; i < 50; i++ {
		m, err := pool.NewPoolMember("fixture:generic-rgb-par", "sha256:deadbeef")
		if err != nil {
			t.Fatalf("NewPoolMember (%d): %v", i, err)
		}
		many.Members = append(many.Members, m)
	}
	if err := pool.Validate(many); err != nil {
		t.Fatalf("expected a 50-member pool to be valid: %v", err)
	}
	if len(many.Members) != 50 {
		t.Fatalf("expected 50 members, got %d", len(many.Members))
	}
}
