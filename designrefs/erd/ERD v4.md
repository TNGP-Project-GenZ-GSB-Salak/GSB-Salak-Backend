
Account {
	ID uuid pk
	UserID uuid > User.ID
	AccountNumber string
	Type string
	Balance decimal
	Currency string
	CreatedAt timestamp
	UpdatedAt timestamp
	IsPrimaryAccount boolean
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
	TicketStart int
	ID uuid pk
}

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

Badge {
	ImageURL string
	CreatedAt timestamp
	UpdatedAt timestamp
	ID uuid pk
	CategoryID uuid > BadgeCategory.ID
}

HoldingBadge {
	HoldingID uuid > Holding.ID
	BadgeID uuid > Badge.ID
	ID uuid pk
	WeightAtAssignment decimal
	AssignedAt timestamp
}

UserBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.ID
}

UserDefaultBadge {
	UserID uuid > User.ID
	BadgeID uuid > Badge.ID
	CreatedAt timestamp
}

User {
	ID uuid pk
	Username string
	CreatedAt timestamp
	UpdatedAt timestamp
	PasswordHash string
	FullName string
}

BadgeCategory {
	Name string
	CreatedAt timestamp
	UpdatedAt timestamp
	ID uuid pk
}

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

KapookGoal {
	ID uuid pk
	GoalAmount decimal
	SavingAmount decimal
	AccountID uuid > Account.ID
	IsActive boolean
	SalakAmount decimal
}

TermConditionAcceptance {
	ID uuid pk
	UserID uuid > User.ID
	AcceptDate timestamp
	CreatedAt timestamp
	UpdatedAt timestamp
}

