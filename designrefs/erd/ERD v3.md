User {
	ID uuid pk
	Username string
	FullName string
	CreatedAt timestamp
	UpdatedAt timestamp
	PasswordHash string
}

// Type: 'savings' | 'salak' | 'kapook'. 'kapook' is new in v3 (a Kapook
// goal lives on its own account rather than overlaying an existing savings
// account). Since production already enforces this via a CHECK constraint
// (not a native enum), adding 'kapook' is a single ALTER TABLE ... CHECK
// swap, not a new-type migration.
Account {
	ID uuid pk
	UserID uuid > User.ID
	AccountNumber string
	Type string
	Balance decimal
	Currency string
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

TicketSequence {
	ID int pk
	NextTicketNumber int
	UpdatedAt timestamp
}

// The maturity interest rate applied on a Kapook-funded holding's
// expiration (see KapookTransaction.salak_expiration below) is a real
// rate, but for MVP it's a backend-code constant, not a column here —
// intentional, not an oversight. Revisit if per-product rates are ever
// needed.
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

// Type: 'deposit' | 'withdraw' | 'withdraw_with_fee' | 'buy_salak' |
// 'salak_expiration'.
//   - deposit: money into Kapook (debits SavingsAccountID, credits
//     KapookAccountID). Produces a debit+credit LedgerEntry pair sharing
//     one ReferenceID, like BuySalak's pair. SavingsAccountID is chosen
//     per-transaction — any of the user's savings accounts can be used,
//     not just the KapookProxyAccount default (that default is scoped
//     specifically to buy_salak's fundingAccountID requirement, below).
//   - withdraw: money out of Kapook, no fee. Capped at 2 per calendar year,
//     enforced at withdrawal time by counting existing Type='withdraw' rows
//     for the account within the current calendar year — no separate
//     counter table (Type itself replaces the old IsPremature flag).
//     Produces its own debit+credit LedgerEntry pair too.
//   - withdraw_with_fee: a withdrawal once the 2 free ones for the year are
//     used up. Amount is the pre-fee amount; the 2% fee is derived at read
//     time (Amount * 0.02), not stored as its own field. Produces a
//     debit+credit LedgerEntry pair.
//   - buy_salak: Kapook-side bookkeeping — increases KapookGoal.SalakAmount
//     (see KapookGoal below for the goal deactivation rule this triggers).
//     The actual salak.holdings row and its own LedgerEntry debit/credit
//     pair (reference_type='buy_salak') come from the existing BuySalak
//     flow, which this triggers; this row does NOT produce a second ledger
//     pair of its own. Funds conceptually come from Kapook (KapookGoal.
//     SavingAmount), but BuySalak's fundingAccountID must be savings-type
//     (a kapook-type account fails its existing type validation) — so this
//     call passes a savings account as fundingAccountID (SavingsAccountID
//     on this row, defaulting to KapookProxyAccount's SavingsAccountID
//     unless the user picks a different one for this specific purchase).
//     Recording it as the funding account is deliberate, not just a type
//     workaround: it's also where the salak value + interest is credited
//     back on salak_expiration below — there is no separate payout-account
//     choice.
//   - salak_expiration: Salak bought with Kapook money expires, decreasing
//     KapookGoal.SalakAmount by this transaction's Amount. Sets HoldingID
//     (the holding that expired, used to look up its buy_salak row's
//     SavingsAccountID) and produces a LedgerEntry credit into that same
//     SavingsAccountID — this is not a forfeiture. The "interest if
//     matured" portion of Amount is computed from a backend-code constant
//     rate, not a database column (see SalakProduct's note above).
// KapookAccountID/SavingsAccountID are qualified names (not a bare
// AccountID) because this table references two distinct Account roles in
// a single row — unlike Holding/LedgerEntry/KapookGoal, which each
// reference exactly one, so a bare AccountID there is already unambiguous.
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

// A default savings account, chosen once when the Kapook account is
// created, used as the default fundingAccountID (see the buy_salak bullet
// above) when a buy_salak KapookTransaction triggers the real BuySalak
// flow — BuySalak requires a savings-type funding account, and a
// kapook-type account can't be passed directly. Because Account mirrors
// the legacy core-banking system and can't be altered with a new column,
// this link lives in its own table, same reasoning as the removed
// KapookAccountConnection.
// UNIQUE(KapookAccountID): one default proxy account per Kapook account; a
// savings account may still serve as the proxy for more than one Kapook
// account.
// UNRESOLVED: BuySalak's existing, already-shipped implementation actually
// debits whatever fundingAccountID it's given. Since funds here are
// conceptually from KapookGoal.SavingAmount rather than the proxy
// account's own balance, some mechanism (e.g. an internal top-up transfer
// into SavingsAccountID immediately before calling BuySalak) is needed so
// the debit doesn't silently draw down the user's real savings balance.
// Not yet designed.
KapookProxyAccount {
	ID uuid pk
	KapookAccountID uuid > Account.ID
	SavingsAccountID uuid > Account.ID
	CreatedAt timestamp
}

// KapookGoal is a goal our own domain tracks — unlike Account (a real
// core-banking account), a single kapook-type Account can have many
// KapookGoal rows over its life (one per goal set/reached/replaced), but at
// most one may be active at a time: AccountID is NOT unique, enforce
// instead via a partial unique index, UNIQUE(AccountID) WHERE IsActive.
// IsActive: true = this is the account's current active goal (same
// true/current-one meaning as SalakProduct.IsActive above). It flips to
// false when a buy_salak transaction's Amount fully satisfies GoalAmount
// (the goal is reached and entirely converted to Salak in one purchase) —
// an earlier partial purchase (e.g. the user already has enough for
// SalakProduct's minimum purchase but hasn't reached GoalAmount yet) does
// NOT deactivate the goal.
// SalakAmount: how much of SavingAmount has been converted into a Salak
// purchase so far (confirmed — not vestigial).
KapookGoal {
	ID uuid pk
	GoalAmount decimal
	SavingAmount decimal
	AccountID uuid > Account.ID
	IsActive boolean
	SalakAmount decimal
}

// Single blanket acceptance per user (confirmed) — no version/document
// tracking needed.
TermConditionAcceptance {
	ID uuid pk
	UserID uuid > User.ID
	AcceptDate timestamp
	CreatedAt timestamp
	UpdatedAt timestamp
}

// CategoryID is required — every badge must belong to a category
// (confirmed).
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
// so this is enforceable at the DB level, not just app-side: a default
// badge can never reference one the user doesn't actually own.
UserDefaultBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.ID
	CreatedAt timestamp
}

BadgeCategory {
	ID uuid pk
	Name string
	CreatedAt timestamp
	UpdatedAt timestamp
}
