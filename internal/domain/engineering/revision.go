package engineering

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// RevisionState — жизненный цикл ревизии (DOM-0024).
type RevisionState string

const (
	RevisionDraft      RevisionState = "draft"
	RevisionWorking    RevisionState = "working"
	RevisionCalculated RevisionState = "calculated"
	RevisionValidated  RevisionState = "validated"
	RevisionApproved   RevisionState = "approved"
	RevisionReleased   RevisionState = "released"
	RevisionArchived   RevisionState = "archived"
)

// Revision — неизменяемая ревизия инженерного объекта.
// Опубликованная ревизия никогда не изменяется; любое изменение
// создаёт новую ревизию. Удаление ревизий запрещено (DOM-0024).
type Revision struct {
	ID        string
	Parent    *Revision
	Timestamp time.Time
	Author    string
	Reason    string
	Checksum  string
	State     RevisionState
}

// NewRevision создаёт ревизию со снапшотом-чексуммой.
func NewRevision(author, reason string, snapshot any) (*Revision, error) {
	if author == "" {
		return nil, fmt.Errorf("revision: author is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("revision: reason is required")
	}
	checksum, err := checksum(snapshot)
	if err != nil {
		return nil, err
	}
	return &Revision{
		ID:        newRevisionID(),
		Timestamp: time.Now().UTC(),
		Author:    author,
		Reason:    reason,
		Checksum:  checksum,
		State:     RevisionDraft,
	}, nil
}

// Child создаёт дочернюю ревизию от текущей (неизменяемость родителя).
func (r *Revision) Child(author, reason string, snapshot any) (*Revision, error) {
	if r.State == RevisionReleased {
		return nil, fmt.Errorf("revision %s: released revision is immutable", r.ID)
	}
	child, err := NewRevision(author, reason, snapshot)
	if err != nil {
		return nil, err
	}
	child.Parent = r
	return child, nil
}

// Advance переводит ревизию в следующее допустимое состояние.
// Переходы: Draft→Working→Calculated→Validated→Approved→Released.
func (r *Revision) Advance() error {
	next := map[RevisionState]RevisionState{
		RevisionDraft:      RevisionWorking,
		RevisionWorking:    RevisionCalculated,
		RevisionCalculated: RevisionValidated,
		RevisionValidated:  RevisionApproved,
		RevisionApproved:   RevisionReleased,
	}
	target, ok := next[r.State]
	if !ok {
		return fmt.Errorf("revision %s: cannot advance from state %s", r.ID, r.State)
	}
	r.State = target
	return nil
}

// Release публикует ревизию, делая её неизменяемой.
// Проходит весь цепочечный переход до состояния Released.
func (r *Revision) Release() error {
	for r.State != RevisionReleased {
		if err := r.Advance(); err != nil {
			return err
		}
	}
	return nil
}

var revisionCounter uint64

func newRevisionID() string {
	revisionCounter++
	h := sha256.New()
	fmt.Fprintf(h, "rev-%d-%d", time.Now().UnixNano(), revisionCounter)
	return "REV-" + hex.EncodeToString(h.Sum(nil)[:6])[:6]
}

// checksum формирует детерминированный SHA-256 снапшота.
func checksum(v any) (string, error) {
	h := sha256.New()
	switch t := v.(type) {
	case string:
		_, _ = h.Write([]byte(t))
	case []byte:
		_, _ = h.Write(t)
	default:
		_, _ = fmt.Fprintf(h, "%+v", t)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
