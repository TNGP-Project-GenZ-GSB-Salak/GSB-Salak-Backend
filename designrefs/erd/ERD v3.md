User {
	ID uuid pk
	Username string
	Fullname string
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
	AccountID uuid > Account.ID
	ProductID uuid > SalakProduct.ID
	Units int
	TicketLetter string
	TicketEnd int
	PurchaseAmount decimal
	PurchaseDate timestamp
	MaturityDate timestamp
	CreatedAt timestamp
	ID uuid pk
	TicketStart int
}

// Type: 'debit' | 'credit'.
// ReferenceType: 'buy_salak' | 'saving_kapook'. ReferenceID is a bare,
// polymorphic uuid with no FK constraint by design — it points at whichever
// table ReferenceType names (salak.holdings' companion is HoldingID above;
// TransactionSavingKapook's companion is its own ID, looked up via
// ReferenceID + ReferenceType='saving_kapook'). This column already ships
// with no CHECK constraint restricting its values, so adding
// 'saving_kapook' needs no production migration at all.
LedgerEntry {
	AccountID uuid > Account.ID
	HoldingID uuid > Holding.ID
	Type string
	Amount decimal
	BalanceAfter decimal
	ReferenceType string
	ReferenceID uuid
	Description string
	CreatedAt timestamp
	ID uuid pk
}

TicketSequence {
	ID int pk
	NextTicketNumber int
	UpdatedAt timestamp
}

// The maturity interest rate applied on a Kapook-funded holding's
// expiration (see TransactionSavingKapook.salak_expiration below) is a real
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
//   - deposit: money into Kapook (debits SourceAccountID, credits
//     KapookAccountID). Produces a debit+credit LedgerEntry pair sharing
//     one ReferenceID, like BuySalak's pair.
//   - withdraw: money out of Kapook, no fee. Capped at 2 per calendar year,
//     enforced at withdrawal time by counting existing Type='withdraw' rows
//     for the account within the current calendar year — no separate
//     counter table (Type itself replaces the old IsPremature flag).
//     Produces its own debit+credit LedgerEntry pair too.
//   - withdraw_with_fee: a withdrawal once the 2 free ones for the year are
//     used up. Amount is the pre-fee amount; the 2% fee is derived at read
//     time (Amount * 0.02), not stored as its own field. Produces a
//     debit+credit LedgerEntry pair.
//   - buy_salak: Kapook-side bookkeeping — increases SavingKapook.SalakAmount
//     (see SavingKapook below for the goal deactivation rule this
//     triggers). The actual salak.holdings row and its own LedgerEntry
//     debit/credit pair (reference_type='buy_salak') come from the
//     existing BuySalak flow, which this triggers; this row does NOT
//     produce a second ledger pair of its own. Sets HoldingID (the holding
//     just minted) and PayoutAccountID (the savings account the user chose
//     to receive this holding's value at expiration — must belong to the
//     same user and be savings-type; reopens the earlier "no HoldingID
//     needed" call now that a per-holding payout destination has to be
//     tracked somewhere).
//   - salak_expiration: Salak bought with Kapook money expires, decreasing
//     SavingKapook.SalakAmount by this transaction's Amount. Sets HoldingID
//     (the holding that expired, used to look up its buy_salak row's
//     PayoutAccountID) and produces a LedgerEntry credit into that
//     PayoutAccountID — this is not a forfeiture. The "interest if matured"
//     portion of Amount is computed from a backend-code constant rate, not
//     a database column (see SalakProduct's note above).
TransactionSavingKapook {
	Type string
	Amount decimal
	UpdatedAt timestamp
	CreatedAt timestamp
	KapookAccountID uuid > Account.ID
	SourceAccountID uuid > Account.ID
	HoldingID uuid > Holding.ID
	PayoutAccountID uuid > Account.ID
	ID uuid pk
}

// SavingKapook is a goal our own domain tracks — unlike Account (a real
// core-banking account), a single kapook-type Account can have many
// SavingKapook rows over its life (one per goal set/reached/replaced), but
// at most one may be active at a time: AccountID is NOT unique, enforce
// instead via a partial unique index, UNIQUE(AccountID) WHERE Status.
// Status: true = this is the account's current active goal. It flips to
// false when a buy_salak transaction's Amount fully satisfies GoalAmount
// (the goal is reached and entirely converted to Salak in one purchase) —
// an earlier partial purchase (e.g. the user already has enough for
// SalakProduct's minimum purchase but hasn't reached GoalAmount yet) does
// NOT deactivate the goal.
// SalakAmount: how much of SavingAmount has been converted into a Salak
// purchase so far (confirmed — not vestigial).
SavingKapook {
	ID uuid pk
	GoalAmount decimal
	SavingAmount decimal
	AccountID uuid > Account.ID
	Status boolean
	SalakAmount decimal
}

// Because Account mirrors the legacy core-banking system and can't be
// altered with a new column, the Kapook<->savings-account link (chosen
// once, at Kapook account creation) lives in its own table.
// UNIQUE(KapookAccountID): each Kapook account links to exactly one savings
// account, but a savings account may back multiple Kapook accounts (e.g.
// separate "Car" and "Vacation" goals funded from the same paycheck
// account).
KapookAccountConnection {
	ID uuid pk
	KapookAccountID uuid > Account.ID
	SavingsAccountID uuid > Account.ID
	CreatedAt timestamp
}

// Single blanket acceptance per user (confirmed) — no version/document
// tracking needed.
AcceptTermCondition {
	ID uuid pk
	UserID uuid > User.ID
	AcceptDate timestamp
	CreatedAt timestamp
	UpdatedAt timestamp
}

// CollectionID is required — every badge must belong to a category
// (confirmed).
Badge {
	ID uuid pk
	ImageURL string
	CollectionID uuid > BadgeCategory.ID
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
	Name string
	CreatedAt timestamp
	UpdatedAt timestamp
	ID uuid pk
}
