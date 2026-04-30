// Package policy compiles Pkl-declared policies into a runtime artifact and
// evaluates Authorize requests against it.
package policy

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
)

type Verb string

const (
	VerbRead  Verb = "read"
	VerbCall  Verb = "call"
	VerbWrite Verb = "write"
	VerbAdmin Verb = "admin"
)

var AllVerbs = []Verb{VerbRead, VerbCall, VerbWrite, VerbAdmin}

type RoleSlug string
type ActionKey string // "ServiceName.MethodName"
type AreaSlug string
type SelectorHash string

// RoleVerb is the composite key for the action allowlist.
type RoleVerb struct {
	Role RoleSlug
	Verb Verb
}

type Compiled struct {
	Generation      uint64
	ActionAllowlist map[RoleVerb]map[ActionKey]struct{}
	AllowRules      map[RoleSlug][]CompiledRule
	DenyRules       map[RoleSlug][]CompiledRule
	AreaExpansion   map[SelectorHash]map[AreaSlug]struct{}
	// RoleInheritance: role → set of roles it transitively inherits (incl. self).
	RoleInheritance map[RoleSlug]map[RoleSlug]struct{}
}

type CompiledRule struct {
	PolicyName string
	Verbs      map[Verb]struct{}   // empty ⇒ all verbs
	Services   map[string]struct{} // empty ⇒ all services
	Targets    CompiledSelector
}

type CompiledSelector struct {
	Hash      SelectorHash
	AreaSet   map[AreaSlug]struct{} // post-expansion; empty ⇒ no area constraint
	ClassSet  map[string]struct{}   // empty ⇒ no class constraint
	EntitySet map[string]struct{}   // empty ⇒ no entity_id constraint
	MatchAny  bool                  // shortcut for AnyEntity
}

// permitsAction is the precomputed-allowlist fast path.
func (c *Compiled) permitsAction(role RoleSlug, verb Verb, key ActionKey) bool {
	if c == nil {
		return false
	}
	set, ok := c.ActionAllowlist[RoleVerb{Role: role, Verb: verb}]
	if !ok {
		return false
	}
	_, ok = set[key]
	return ok
}

// HashSelector computes a stable structural hash of a raw selector.
func HashSelector(areas, classes, entityIDs []string) SelectorHash {
	h := sha256.New()
	write := func(label string, items []string) {
		sorted := append([]string(nil), items...)
		sort.Strings(sorted)
		h.Write([]byte(label))
		h.Write([]byte{0})
		for _, s := range sorted {
			h.Write([]byte(s))
			h.Write([]byte{0})
		}
		h.Write([]byte{1})
	}
	write("a", areas)
	write("c", classes)
	write("e", entityIDs)
	var sz [8]byte
	binary.BigEndian.PutUint64(sz[:], uint64(len(areas)+len(classes)+len(entityIDs)))
	h.Write(sz[:])
	return SelectorHash(hex.EncodeToString(h.Sum(nil)))
}

// MakeActionKey is a convenience for (service, method) → ActionKey.
func MakeActionKey(service, method string) ActionKey {
	return ActionKey(service + "." + method)
}
