// Type: 'savings' | 'salak' | 'kapook'. Since production already enforces
// this via a CHECK constraint (not a native enum), adding 'kapook' is a
// single ALTER TABLE ... CHECK swap, not a new-type migration.
// IsPrimaryAccount: new in v4. The user's primary account is where matured
// Salak proceeds (value + interest) are credited on a salak_expiration
// KapookTransaction — which is what made v3's per-Kapook proxy-account link
// table unnecessary, since the destination is now derivable from the user
// rather than configured per Kapook account.
// Should be exactly one per user and savings-type (a salak- or kapook-type
// account isn't a meaningful payout destination): enforce with a partial
// unique index, UNIQUE(UserID) WHERE IsPrimaryAccount, plus a type check.
Account {
	ID uuid pk
	UserID uuid > User.ID
	AccountNumber string
	Type string
	Balance decimal
	Currency string
	IsPrimaryAccount boolean
	CreatedAt timestamp
	UpdatedAt timestamp
}

Holding {
	ID uuid pk
	AccountID uuid > Account.ID
	ProductID uuid > SalakProduct.ID
	Units int
	TicketStart int
	TicketEnd int
	TicketLetter string
	PurchaseAmount decimal
	PurchaseDate timestamp
	MaturityDate timestamp
	CreatedAt timestamp
}

// Type: 'debit' | 'credit'.
// ReferenceType: 'buy_salak' | 'kapook_transaction'. ReferenceID is a bare,
// polymorphic uuid with no FK constraint by design — it points at whichever
// table ReferenceType names (salak.holdings' companion is HoldingID above;
// KapookTransaction's companion is its own ID, looked up via ReferenceID +
// ReferenceType='kapook_transaction'). This column already ships with no
// CHECK constraint restricting its values, so adding 'kapook_transaction'
// needs no production migration at all.
LedgerEntry {
	ID uuid pk
	AccountID uuid > Account.ID
	HoldingID uuid > Holding.ID
	Type string
	Amount decimal
	BalanceAfter decimal
	ReferenceType string
	ReferenceID uuid
	Description string
	CreatedAt timestamp
}

// The ticket space is the ordered product of a Thai consonant and a 7-digit
// number: ก0000000 → ก9999999 → ข0000000 → ... → ฮ9999999. The cursor into
// that space is therefore compound — (NextTicketLetter, NextTicketNumber) —
// because once the number component is bounded to 0..9999999 and rolls over
// into the next letter, a number alone no longer identifies a position.
// Letter set: the 44 true Thai consonants. U+0E01..U+0E2E is 46 code points,
// but ฤ (U+0E24) and ฦ (U+0E26) are independent vowels, not consonants, and
// are excluded — so advancing the letter is a skip over those two, not a
// plain +1 offset from U+0E01. Total capacity 44 × 10,000,000 = 440,000,000
// tickets.
// Rollover: a holding's range never crosses a letter boundary. If the
// current letter block has too few numbers left to fit the whole purchase,
// the cursor jumps to the next letter's 0000000 and allocates there,
// abandoning the leftover tail (at most Units-1 tickets, once per 10M).
// This is what keeps Holding's single TicketLetter column — and its
// TicketEnd - TicketStart + 1 = Units check — valid as-is.
// Still a single row (the ID=1 singleton), locked SELECT ... FOR UPDATE
// while a range is reserved. ID stays the primary key so the lock target is
// stable; making (letter, number) the literal PK would mean mutating the PK
// on every allocation, and the existing repository locks WHERE id = 1.
// NOTE: this supersedes the shipped implementation, where the letter is
// picked at random per holding (randomTicketLetter, drawing from all 46 code
// points) and the number is an unbounded counter that never rolls over.
TicketSequence {
	ID int pk
	NextTicketLetter string
	NextTicketNumber int
	UpdatedAt timestamp
}

// The maturity interest rate applied on a Kapook-funded holding's expiration
// (see KapookTransaction.salak_expiration below) is a real rate, but for MVP
// it's a backend-code constant, not a column here — intentional, not an
// oversight. Revisit if per-product rates are ever needed.
SalakProduct {
	ID uuid pk
	Code string
	Name string
	TermMonths int
	UnitPrice decimal
	MinPurchase decimal
	MaxPurchase decimal
	StepAmount decimal
	IsActive bool
	CreatedAt timestamp
	UpdatedAt timestamp
}

// CategoryID is required — every badge must belong to a category.
Badge {
	ID uuid pk
	ImageURL string
	CategoryID uuid > BadgeCategory.ID
	CreatedAt timestamp
	UpdatedAt timestamp
}

// HoldingID is UNIQUE (one immutable badge per holding, never reassigned).
// WeightAtAssignment snapshots Badge.Weight at the moment it was rolled, so
// a later change to the badge catalog's weights doesn't retroactively alter
// which badge a past holding appears to have "deserved."
HoldingBadge {
	ID uuid pk
	HoldingID uuid > Holding.ID
	BadgeID uuid > Badge.ID
	WeightAtAssignment decimal
	AssignedAt timestamp
}

UserBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.ID
}

// (UserID, BadgeID) should be a composite FK to UserBadge(UserID, BadgeID) —
// UserBadge already has a UNIQUE(UserID, BadgeID) constraint in production,
// so this is enforceable at the DB level, not just app-side: a default badge
// can never reference one the user doesn't actually own.
UserDefaultBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.ID
	CreatedAt timestamp
}

User {
	ID uuid pk
	Username string
	PasswordHash string
	FullName string
	CreatedAt timestamp
	UpdatedAt timestamp
}

BadgeCategory {
	ID uuid pk
	Name string
	CreatedAt timestamp
	UpdatedAt timestamp
}

// Type: 'deposit' | 'withdraw' | 'withdraw_with_fee' | 'buy_salak' |
// 'salak_expiration'.
//   - deposit: money into Kapook (debits SavingsAccountID, credits
//     KapookAccountID). Produces a debit+credit LedgerEntry pair sharing one
//     ReferenceID, like BuySalak's pair. SavingsAccountID is chosen
//     per-transaction — any of the user's savings accounts can be used, not
//     only the primary one.
//   - withdraw: money out of Kapook, no fee. Capped at 2 per calendar year,
//     enforced at withdrawal time by counting existing Type='withdraw' rows
//     for the account within the current calendar year — no separate counter
//     table. Produces its own debit+credit LedgerEntry pair too.
//   - withdraw_with_fee: a withdrawal once the 2 free ones for the year are
//     used up. Amount is the pre-fee amount; the 2% fee is derived at read
//     time (Amount * 0.02), not stored as its own field. Produces a
//     debit+credit LedgerEntry pair.
//   - buy_salak: Kapook-side bookkeeping — increases KapookGoal.SalakAmount
//     (see KapookGoal below for the goal deactivation rule this triggers).
//     The actual salak.holdings row and its own LedgerEntry debit/credit pair
//     (reference_type='buy_salak') come from the existing BuySalak flow,
//     which this triggers; this row does NOT produce a second ledger pair of
//     its own. Sets HoldingID (the holding just minted).
//     UNRESOLVED: funds conceptually come from Kapook (KapookGoal.
//     SavingAmount), but BuySalak's fundingAccountID must be savings-type (a
//     kapook-type account fails its existing type validation) — and v3's
//     per-Kapook proxy account, which used to supply it, is gone. Either the
//     primary account doubles as fundingAccountID, or SavingsAccountID is
//     set per-purchase here. Whichever it is, BuySalak's shipped code
//     actually debits whatever account it's given, so some mechanism (e.g.
//     an internal top-up transfer immediately before the call) is still
//     needed so the debit doesn't silently draw down the user's real savings
//     balance. Not yet designed.
//   - salak_expiration: Salak bought with Kapook money expires, decreasing
//     KapookGoal.SalakAmount by this transaction's Amount. Sets HoldingID
//     (the holding that expired) and produces a LedgerEntry credit into the
//     owning user's primary account (Account.IsPrimaryAccount) — this is not
//     a forfeiture, and needs no PayoutAccountID column since the
//     destination is derivable from the user. The "interest if matured"
//     portion of Amount is computed from a backend-code constant rate, not a
//     database column (see SalakProduct's note above).
// KapookAccountID/SavingsAccountID are qualified names (not a bare
// AccountID) because this table references two distinct Account roles in a
// single row — unlike Holding/LedgerEntry/KapookGoal, which each reference
// exactly one, so a bare AccountID there is already unambiguous.
KapookTransaction {
	ID uuid pk
	Type string
	Amount decimal
	KapookAccountID uuid > Account.ID
	SavingsAccountID uuid > Account.ID
	HoldingID uuid > Holding.ID
	CreatedAt timestamp
	UpdatedAt timestamp
}

// KapookGoal is a goal our own domain tracks — unlike Account (a real
// core-banking account), a single kapook-type Account can have many
// KapookGoal rows over its life (one per goal set/reached/replaced), but at
// most one may be active at a time: AccountID is NOT unique, enforce instead
// via a partial unique index, UNIQUE(AccountID) WHERE IsActive.
// IsActive: true = this is the account's current active goal (same
// true/current-one meaning as SalakProduct.IsActive above). It flips to
// false when a buy_salak transaction's Amount fully satisfies GoalAmount
// (the goal is reached and entirely converted to Salak in one purchase) — an
// earlier partial purchase (e.g. the user already has enough for
// SalakProduct's minimum purchase but hasn't reached GoalAmount yet) does
// NOT deactivate the goal.
// SalakAmount: how much of SavingAmount has been converted into a Salak
// purchase so far.
KapookGoal {
	ID uuid pk
	GoalAmount decimal
	SavingAmount decimal
	AccountID uuid > Account.ID
	IsActive boolean
	SalakAmount decimal
}

// Single blanket acceptance per user — no version/document tracking needed.
TermConditionAcceptance {
	ID uuid pk
	UserID uuid > User.ID
	AcceptDate timestamp
	CreatedAt timestamp
	UpdatedAt timestamp
}
