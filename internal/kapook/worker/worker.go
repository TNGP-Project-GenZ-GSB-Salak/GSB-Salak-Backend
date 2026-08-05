// Package worker fires each Kapook goal's auto-purchase once its countdown
// has expired - the headline behaviour of the whole Kapook feature: reach
// the target, do nothing, and the system buys the Salak without the
// customer opening the app.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ciaabcdefg/gsb-salak-backend/internal/account"
	accountdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/account/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/kapook"
	kapookdomain "github.com/ciaabcdefg/gsb-salak-backend/internal/kapook/domain"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/platform/clock"
	"github.com/ciaabcdefg/gsb-salak-backend/internal/salak"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// defaultClaimBatchLimit caps how many goals one pass claims, so a huge
// backlog can't make a single RunOnce hold an unbounded number of row
// locks - any goals left over are simply due again (or still due) on the
// next tick.
const defaultClaimBatchLimit = 20

// Outcome classifies what happened to one claimed goal within a pass.
type Outcome string

const (
	// OutcomePurchased means the auto-purchase succeeded.
	OutcomePurchased Outcome = "purchased"
	// OutcomeDeferred means the draw-day guard rejected it - retryable, and
	// left exactly as-is (IsActive true, GoalReachedAt unchanged) so the
	// next tick's claiming query picks it straight back up.
	OutcomeDeferred Outcome = "deferred"
	// OutcomeFailed means anything else went wrong (an inactive product, a
	// funding-balance mismatch, or any other error). It is logged
	// explicitly and, like a deferral, retried next tick - this package
	// makes no attempt to give up on a goal.
	OutcomeFailed Outcome = "failed"
)

// GoalOutcome records what happened to one specific claimed goal - the
// detail behind a Summary's counts, useful for both tests and logging.
type GoalOutcome struct {
	GoalID    uuid.UUID
	AccountID uuid.UUID
	Outcome   Outcome
	Err       error
}

// Summary is RunOnce's report of a single pass over due goals.
type Summary struct {
	Claimed int
	Results []GoalOutcome
}

func (s Summary) Purchased() int { return s.count(OutcomePurchased) }
func (s Summary) Deferred() int  { return s.count(OutcomeDeferred) }
func (s Summary) Failed() int    { return s.count(OutcomeFailed) }

func (s Summary) count(o Outcome) int {
	n := 0
	for _, r := range s.Results {
		if r.Outcome == o {
			n++
		}
	}
	return n
}

// Worker fires each Kapook goal's auto-purchase once its countdown expires.
// It has no leader-election concept - ClaimDueGoals's SELECT ... FOR UPDATE
// SKIP LOCKED is what lets multiple replicas, or two overlapping ticks, run
// RunOnce concurrently and safely: each claims a disjoint subset of due
// goals rather than blocking on or duplicating another's work.
type Worker struct {
	db                *gorm.DB
	goals             kapook.GoalRepository
	accounts          account.Service
	kapookSvc         kapook.Service
	salakSvc          salak.Service
	clk               clock.Clock
	countdownDuration time.Duration
	claimBatchLimit   int
}

func New(db *gorm.DB, goals kapook.GoalRepository, accounts account.Service, kapookSvc kapook.Service, salakSvc salak.Service, clk clock.Clock, countdownDuration time.Duration) *Worker {
	return &Worker{
		db:                db,
		goals:             goals,
		accounts:          accounts,
		kapookSvc:         kapookSvc,
		salakSvc:          salakSvc,
		clk:               clk,
		countdownDuration: countdownDuration,
		claimBatchLimit:   defaultClaimBatchLimit,
	}
}

// RunOnce claims and processes every currently-due goal in a single pass -
// the entry point tests call directly, so no test ever has to sleep
// waiting for a real tick. It always returns a Summary alongside any
// pass-level error (a failed claim, or the surrounding transaction itself
// failing); a per-goal failure never aborts the pass - it's recorded as
// that goal's Outcome and the loop moves on.
func (w *Worker) RunOnce(ctx context.Context) (Summary, error) {
	var summary Summary
	now := w.clk.Now()
	cutoff := now.Add(-w.countdownDuration)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	err := w.db.Transaction(func(tx *gorm.DB) error {
		goals, err := w.goals.ClaimDueGoals(ctx, tx, cutoff, today, w.claimBatchLimit)
		if err != nil {
			return fmt.Errorf("claim due goals: %w", err)
		}
		summary.Claimed = len(goals)

		for _, goal := range goals {
			// Defensive re-check: the claiming query's own WHERE clause
			// already guarantees this under Postgres's FOR UPDATE
			// re-evaluation against the latest committed row version, but
			// asserting it explicitly here documents the invariant rather
			// than resting on that alone.
			if !goal.IsActive || goal.GoalReachedAt == nil {
				continue
			}
			summary.Results = append(summary.Results, w.processGoal(ctx, tx, goal))
		}
		return nil
	})
	if err != nil {
		return summary, err
	}
	return summary, nil
}

// processGoal buys goal's full available balance through
// kapook.Service.BuyFromGoalInTx, inside tx - the same transaction (and row
// lock) ClaimDueGoals already holds, via a savepoint that isolates this
// specific goal's failure. amount is AvailableBalance, not GoalAmount
// directly: if the customer already converted part of the goal manually
// during the countdown, only the remainder is left to buy - using
// GoalAmount unconditionally would overshoot and fail every time.
func (w *Worker) processGoal(ctx context.Context, tx *gorm.DB, goal kapookdomain.Goal) GoalOutcome {
	out := GoalOutcome{GoalID: goal.ID, AccountID: goal.AccountID}

	kapookAcc, err := w.accounts.GetByIDUnscoped(ctx, goal.AccountID)
	if err != nil {
		return w.fail(out, fmt.Errorf("resolve kapook account: %w", err))
	}

	salakAccountID, ok, err := w.findSalakAccount(ctx, kapookAcc.UserID)
	if err != nil {
		return w.fail(out, fmt.Errorf("resolve salak account: %w", err))
	}
	if !ok {
		return w.fail(out, fmt.Errorf("no salak account found for user %s", kapookAcc.UserID))
	}

	amount := goal.AvailableBalance()
	if _, err := w.kapookSvc.BuyFromGoalInTx(ctx, tx, kapookAcc.UserID, goal.AccountID, salakAccountID, amount); err != nil {
		if errors.Is(err, salak.ErrDrawDay) {
			return w.deferGoal(ctx, tx, out, goal)
		}
		return w.fail(out, err)
	}

	out.Outcome = OutcomePurchased
	return out
}

// deferGoal persists goal's retry date - today's draw-day rejection made it
// unactionable, but the client still needs to learn why and when, rather
// than watching a "processing" state for the whole draw day. Computing the
// product just to find that date, and persisting it, is itself allowed to
// fail (an unreachable product, a DB error) - that's a real fault, not a
// draw day, so it's recorded as OutcomeFailed and retried next tick like
// any other failure, rather than silently left as "deferred" with no date.
func (w *Worker) deferGoal(ctx context.Context, tx *gorm.DB, out GoalOutcome, goal kapookdomain.Goal) GoalOutcome {
	product, err := w.salakSvc.GetProduct(ctx, goal.ProductID)
	if err != nil {
		return w.fail(out, fmt.Errorf("resolve product for draw-day deferral: %w", err))
	}
	until, err := w.salakSvc.NextAvailableDate(ctx, product)
	if err != nil {
		return w.fail(out, fmt.Errorf("compute next available date: %w", err))
	}
	if err := w.goals.SetAutoPurchaseDeferral(ctx, tx, goal.ID, until); err != nil {
		return w.fail(out, fmt.Errorf("persist draw-day deferral: %w", err))
	}
	out.Outcome = OutcomeDeferred
	log.Printf("kapook worker: deferring goal %s (draw day) until %s", goal.ID, until.Format("2006-01-02"))
	return out
}

// fail marks out as failed and logs it explicitly - the goal is left as-is
// (still due) and gets retried next tick, but this is the record of why an
// unattended attempt didn't complete that the ticket's acceptance criteria
// require: never leave a goal past its deadline with nothing having
// happened and no trace of the reason.
func (w *Worker) fail(out GoalOutcome, err error) GoalOutcome {
	out.Outcome = OutcomeFailed
	out.Err = err
	log.Printf("ERROR: kapook worker: auto-purchase failed for goal %s (account %s): %v", out.GoalID, out.AccountID, err)
	return out
}

// findSalakAccount resolves the one salak-type account this backend
// provisions per user (there is no create-account flow, so a user has
// exactly the accounts cmd/seed or registration gave them) - the worker
// has no live customer choice to defer to, unlike the HTTP-driven
// BuyFromGoal, which takes salakAccountID from the request.
func (w *Worker) findSalakAccount(ctx context.Context, userID uuid.UUID) (uuid.UUID, bool, error) {
	accounts, err := w.accounts.ListByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, false, err
	}
	for _, a := range accounts {
		if a.Type == accountdomain.TypeSalak {
			return a.ID, true, nil
		}
	}
	return uuid.Nil, false, nil
}

// Run polls RunOnce every tickInterval until ctx is cancelled, logging each
// pass's summary (when it did anything) and any pass-level error. This is
// cmd/worker's long-running mode; tests call RunOnce directly instead.
func (w *Worker) Run(ctx context.Context, tickInterval time.Duration) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			summary, err := w.RunOnce(ctx)
			if err != nil {
				log.Printf("ERROR: kapook worker: pass failed: %v", err)
				continue
			}
			if summary.Claimed > 0 {
				log.Printf("kapook worker: claimed %d, purchased %d, deferred %d, failed %d", summary.Claimed, summary.Purchased(), summary.Deferred(), summary.Failed())
			}
		}
	}
}
